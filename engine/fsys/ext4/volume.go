package ext4

import (
	"context"
	"fmt"
	"io"
	"sort"
	"sync"

	"github.com/mehran9675/remanence/engine/fsys"
)

func init() { fsys.RegisterParser(Open, "ext2", "ext3", "ext4") }

const rootInode = 2

type volume struct {
	r      io.ReaderAt
	devOff int64
	s      superblock
	groups []group
	j      *journal
	v      *fsys.Volume

	bitmapsMu sync.Mutex
	bitmaps   map[int64]fsys.Bitmap
	dirs      map[uint32]bool // directory inodes walked
	placed    map[uint32]bool // file inodes in the tree
}

// Open parses the ext2/3/4 volume in r.
func Open(ctx context.Context, r io.ReaderAt, devOff, size int64, progress fsys.Progress) (*fsys.Volume, error) {
	sb := make([]byte, 1024)
	r.ReadAt(sb, 1024)
	s, err := parseSuperblock(sb)
	if err != nil {
		// The first backup is in group 1; where that is depends on the
		// block size, so try the usual layouts.
		for _, off := range []int64{32768 * 4096, 8193 * 1024, 16384 * 2048 * 1} {
			if _, e := r.ReadAt(sb, off); e == nil {
				if s, err = parseSuperblock(sb); err == nil {
					break
				}
			}
		}
		if err != nil {
			return nil, err
		}
	}
	vol := &volume{r: r, devOff: devOff, s: s, bitmaps: map[int64]fsys.Bitmap{}, dirs: map[uint32]bool{}, placed: map[uint32]bool{}}
	if vol.groups, err = readGroups(r, &s); err != nil {
		return nil, fmt.Errorf("ext4: group descriptors: %w", err)
	}
	vol.v = fsys.NewVolume(s.fsName(), devOff, size)
	vol.v.Label = s.label
	vol.v.ClusterSize = s.blockSize
	vol.j = vol.readJournal()
	if vol.j == nil && s.compat&compatHasJournal != 0 {
		vol.v.Warn("the journal couldn't be read; deleted files on ext3/ext4 can only be found by signature")
	}

	root, err := vol.readInode(rootInode)
	if err != nil {
		return nil, err
	}
	vol.dirs[rootInode] = true
	vol.v.Root.ID = rootInode
	vol.v.Root.Modified = root.mtime
	vol.walk(ctx, vol.v.Root, root, false, 0)
	if progress != nil {
		progress(0.6)
	}
	if fsys.IsDeep(ctx) {
		vol.findOrphans(ctx, progress)
	}
	if progress != nil {
		progress(1)
	}
	vol.v.Sort()
	return vol.v, nil
}

func (vol *volume) readBlock(blk int64) ([]byte, error) {
	if blk < 0 || blk >= vol.s.blocks {
		return nil, fmt.Errorf("ext4: block %d out of range", blk)
	}
	b := make([]byte, vol.s.blockSize)
	if _, err := vol.r.ReadAt(b, blk*vol.s.blockSize); err != nil && err != io.EOF {
		return nil, err
	}
	return b, nil
}

// inodeAt locates inode n: the block holding it and the offset inside.
func (vol *volume) inodeAt(n uint32) (blk, off int64, ok bool) {
	if n == 0 || n > vol.s.inodes {
		return 0, 0, false
	}
	g := int64(n-1) / int64(vol.s.inodesPerGroup)
	if g >= int64(len(vol.groups)) {
		return 0, 0, false
	}
	idx := int64(n-1) % int64(vol.s.inodesPerGroup)
	pos := vol.groups[g].inodeTable*vol.s.blockSize + idx*vol.s.inodeSize
	return pos / vol.s.blockSize, pos % vol.s.blockSize, true
}

func (vol *volume) readInode(n uint32) (inode, error) {
	blk, off, ok := vol.inodeAt(n)
	if !ok {
		return inode{}, fmt.Errorf("ext4: no inode %d", n)
	}
	b, err := vol.readBlock(blk)
	if err != nil {
		return inode{}, err
	}
	return parseInode(b[off : off+vol.s.inodeSize]), nil
}

// journaledInode finds the newest copy of inode n in the journal from
// before it was deleted: still linked and with a block map. When the
// inode is in use again by another file, only a copy of an earlier
// generation counts.
func (vol *volume) journaledInode(n uint32, cur inode) (inode, bool) {
	blk, off, ok := vol.inodeAt(n)
	if !ok {
		return inode{}, false
	}
	for _, b := range vol.versions(blk) {
		in := parseInode(b[off : off+vol.s.inodeSize])
		if !in.live() || in.mode == 0 {
			continue
		}
		if cur.live() && in.generation == cur.generation {
			continue
		}
		if ext, err := in.mapping(vol.readBlock, &vol.s); len(ext) > 0 || (err == nil && in.size == 0) {
			return in, true
		}
	}
	return inode{}, false
}

// resolve picks the inode version to recover a name from. ok is false
// when the name must be left out: it's an old name of a file that still
// exists (renamed or moved), not a deleted one.
func (vol *volume) resolve(n uint32, deleted bool) (in inode, state fsys.State, note string, ok bool) {
	cur, err := vol.readInode(n)
	if err != nil {
		return inode{}, 0, "", false
	}
	if !deleted && cur.live() {
		return cur, fsys.Existing, "", true
	}
	if old, found := vol.journaledInode(n, cur); found {
		return old, fsys.Deleted, "Rebuilt from the journal", true
	}
	if cur.live() {
		return inode{}, 0, "", false
	}
	if cur.mode != 0 {
		if ext, _ := cur.mapping(vol.readBlock, &vol.s); len(ext) > 0 {
			return cur, fsys.Deleted, "", true // ext2 keeps the block map
		}
	}
	return cur, fsys.Deleted, "ext4 erased where the data was; look among the files found by signature", true
}

func (vol *volume) entry(n uint32, name string, in inode, state fsys.State, note string) *fsys.Entry {
	e := &fsys.Entry{
		ID:       uint64(n),
		Name:     name,
		IsDir:    in.isDir(),
		Created:  in.crtime,
		Modified: in.mtime,
		Accessed: in.atime,
		State:    state,
		Note:     note,
		Health:   100,
	}
	if e.IsDir {
		return e
	}
	e.Size = in.size
	if in.flags&flagInlineData != 0 {
		e.Inline = append([]byte(nil), in.block[:min(int64(60), in.size)]...)
		return e
	}
	ext, err := in.mapping(vol.readBlock, &vol.s)
	if err != nil && len(ext) == 0 {
		if e.Size > 0 {
			e.Health = 0
		}
		return e
	}
	e.Runs = vol.runs(ext, in.size)
	if state != fsys.Existing {
		e.Health = fsys.Health(e.Runs, vol.s.blockSize, vol.devOff, vol.usedBlock)
		if e.Health < 100 && e.Note == "" {
			e.Note = "Some blocks have been reused since deletion"
		}
	}
	return e
}

// runs converts extents to device runs, with sparse runs for holes.
func (vol *volume) runs(ext []extent, size int64) []fsys.Run {
	sort.Slice(ext, func(i, j int) bool { return ext[i].lblk < ext[j].lblk })
	bs := vol.s.blockSize
	var out []fsys.Run
	next := int64(0)
	for _, e := range ext {
		if e.lblk > next {
			out = append(out, fsys.Run{Offset: -1, Length: (e.lblk - next) * bs})
		}
		if e.lblk+e.len <= next {
			continue
		}
		out = append(out, fsys.Run{Offset: vol.devOff + e.pblk*bs, Length: e.len * bs})
		next = e.lblk + e.len
		if next*bs >= size {
			break
		}
	}
	return out
}

func (vol *volume) usedBlock(blk int64) bool {
	s := &vol.s
	g := (blk - s.firstDataBlock) / s.blocksPerGroup
	if g < 0 || g >= int64(len(vol.groups)) {
		return true
	}
	vol.bitmapsMu.Lock()
	bm, ok := vol.bitmaps[g]
	vol.bitmapsMu.Unlock()
	if !ok {
		if vol.groups[g].flags&bgBlockUninit == 0 {
			bm, _ = vol.readBlock(vol.groups[g].blockBitmap)
		} else {
			bm = make(fsys.Bitmap, s.blockSize)
		}
		vol.bitmapsMu.Lock()
		vol.bitmaps[g] = bm
		vol.bitmapsMu.Unlock()
	}
	return bm.Used((blk - s.firstDataBlock) % s.blocksPerGroup)
}

// dirBlocks reads a directory's blocks through its block map, and the
// older copies of them the journal still holds.
func (vol *volume) dirBlocks(in inode) (cur, old [][]byte) {
	if in.flags&flagInlineData != 0 {
		return [][]byte{append([]byte(nil), in.block[4:]...)}, nil
	}
	ext, _ := in.mapping(vol.readBlock, &vol.s)
	for _, e := range ext {
		for i := int64(0); i < e.len && len(cur) < 65536; i++ {
			if b, err := vol.readBlock(e.pblk + i); err == nil {
				cur = append(cur, b)
			}
			old = append(old, vol.versions(e.pblk+i)...)
		}
	}
	return cur, old
}

func (vol *volume) walk(ctx context.Context, dir *fsys.Entry, in inode, deleted bool, depth int) {
	if depth > 64 || ctx.Err() != nil {
		return
	}
	filetype := vol.s.incompat&incompatFiletype != 0
	var names []dirEntry
	cur, old := vol.dirBlocks(in)
	for _, b := range cur {
		names = append(names, parseDirBlock(b, vol.s.inodes, filetype)...)
	}
	// Since Linux 5.13 a removal also wipes the entry, so the names of
	// deleted files often survive only in the journal's older copies of
	// the block.
	seen := map[dirEntry]bool{}
	for _, d := range names {
		seen[dirEntry{inode: d.inode, name: d.name}] = true
	}
	for _, b := range old {
		for _, d := range parseDirBlock(b, vol.s.inodes, filetype) {
			if k := (dirEntry{inode: d.inode, name: d.name}); !seen[k] {
				seen[k] = true
				d.deleted = true
				names = append(names, d)
			}
		}
	}
	// Live names first, so a deleted old name of a file that was moved
	// here doesn't get placed before the real one.
	sort.SliceStable(names, func(i, j int) bool { return !names[i].deleted && names[j].deleted })
	for _, d := range names {
		if d.name == "." || d.name == ".." || d.fileType == ftSymlink {
			continue
		}
		if vol.placed[d.inode] || vol.dirs[d.inode] {
			continue
		}
		child, state, note, ok := vol.resolve(d.inode, d.deleted || deleted)
		if !ok {
			continue
		}
		if child.mode&modeTypeMask == modeSymlink {
			continue
		}
		if child.mode == 0 && d.fileType == ftDir {
			child.mode = modeDir // a wiped inode: trust the entry's type
		}
		e := vol.entry(d.inode, d.name, child, state, note)
		dir.Add(e)
		if e.IsDir {
			vol.dirs[d.inode] = true
			vol.walk(ctx, e, child, state != fsys.Existing, depth+1)
		} else {
			vol.placed[d.inode] = true
		}
	}
}

// findOrphans reads every inode table for deleted files no folder names
// any more and that the journal or an ext2 block map can still place.
func (vol *volume) findOrphans(ctx context.Context, progress fsys.Progress) {
	s := &vol.s
	perBlock := s.blockSize / s.inodeSize
	for g := range vol.groups {
		if ctx.Err() != nil {
			return
		}
		tableBlocks := (int64(s.inodesPerGroup)*s.inodeSize + s.blockSize - 1) / s.blockSize
		for tb := int64(0); tb < tableBlocks; tb++ {
			b, err := vol.readBlock(vol.groups[g].inodeTable + tb)
			if err != nil {
				continue
			}
			for i := int64(0); i < perBlock; i++ {
				n := uint32(int64(g)*int64(s.inodesPerGroup) + tb*perBlock + i + 1)
				if n < s.firstIno || n > s.inodes || vol.placed[n] || vol.dirs[n] {
					continue
				}
				cur := parseInode(b[i*s.inodeSize : (i+1)*s.inodeSize])
				if cur.live() || cur.dtime == 0 || !(cur.isFile() || cur.mode == 0) {
					continue
				}
				in, state, note, ok := vol.resolve(n, true)
				if !ok || !in.isFile() || in.size == 0 {
					continue
				}
				e := vol.entry(n, fmt.Sprintf("inode %d", n), in, state, note)
				if len(e.Runs) == 0 && len(e.Inline) == 0 {
					continue
				}
				e.State = fsys.Orphan
				vol.placed[n] = true
				vol.v.Lost.Add(e)
			}
		}
		if progress != nil {
			progress(0.6 + 0.4*float64(g+1)/float64(len(vol.groups)))
		}
	}
}
