package ext4

import (
	"sort"

	"github.com/xmhrnx/remanance/engine/fsys"
)

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
