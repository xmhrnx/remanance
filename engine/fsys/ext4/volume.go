package ext4

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/xmhrnx/remanance/engine/fsys"
)

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

func init() { fsys.RegisterParser(Open, "ext2", "ext3", "ext4") }

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
