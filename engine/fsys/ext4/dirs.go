package ext4

import (
	"context"
	"fmt"
	"sort"

	"github.com/xmhrnx/remanance/engine/fsys"
)

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
