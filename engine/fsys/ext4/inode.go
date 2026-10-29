package ext4

import (
	"encoding/binary"
	"errors"
	"time"
)

const (
	modeTypeMask = 0xF000
	modeDir      = 0x4000
	modeFile     = 0x8000
	modeSymlink  = 0xA000

	flagExtents    = 0x00080000
	flagInlineData = 0x10000000

	extentMagic = 0xF30A
)

type inode struct {
	mode       uint16
	size       int64
	atime      time.Time
	ctime      time.Time
	mtime      time.Time
	crtime     time.Time
	dtime      uint32
	links      uint16
	flags      uint32
	generation uint32
	block      [60]byte
}

func (i *inode) isDir() bool  { return i.mode&modeTypeMask == modeDir }
func (i *inode) isFile() bool { return i.mode&modeTypeMask == modeFile }
func (i *inode) live() bool   { return i.links > 0 && i.dtime == 0 }

func unix(v uint32) time.Time {
	if v == 0 {
		return time.Time{}
	}
	return time.Unix(int64(v), 0).UTC()
}

func parseInode(b []byte) inode {
	le := binary.LittleEndian
	in := inode{
		mode:       le.Uint16(b[0:]),
		size:       int64(le.Uint32(b[4:])),
		atime:      unix(le.Uint32(b[8:])),
		ctime:      unix(le.Uint32(b[12:])),
		mtime:      unix(le.Uint32(b[16:])),
		dtime:      le.Uint32(b[20:]),
		links:      le.Uint16(b[26:]),
		flags:      le.Uint32(b[32:]),
		generation: le.Uint32(b[100:]),
	}
	copy(in.block[:], b[40:100])
	in.size |= int64(le.Uint32(b[108:])) << 32
	if len(b) >= 0x94 {
		extra := int(le.Uint16(b[0x80:]))
		if extra >= 0x14 && 0x80+extra <= len(b) {
			in.crtime = unix(le.Uint32(b[0x90:]))
		}
	}
	return in
}

// extent maps len blocks of a file from logical block lblk to physical
// block pblk.
type extent struct {
	lblk, pblk, len int64
}

var errBadMap = errors.New("ext4: damaged block map")

// blockReader reads one file system block; the journal-aware volume
// reader serves older copies of metadata blocks through it.
type blockReader func(blk int64) ([]byte, error)

// mapping returns a file's extents, from its extent tree or its ext2/3
// block pointers.
func (in *inode) mapping(read blockReader, s *superblock) ([]extent, error) {
	if in.flags&flagExtents != 0 {
		return extentTree(in.block[:], read, s, 0)
	}
	return blockMap(in, read, s)
}

func extentTree(node []byte, read blockReader, s *superblock, depth int) ([]extent, error) {
	le := binary.LittleEndian
	if len(node) < 12 || le.Uint16(node) != extentMagic || depth > 5 {
		return nil, errBadMap
	}
	entries := int(le.Uint16(node[2:]))
	max := int(le.Uint16(node[4:]))
	level := int(le.Uint16(node[6:]))
	if entries > max || 12+12*entries > len(node) {
		return nil, errBadMap
	}
	var out []extent
	for i := 0; i < entries; i++ {
		e := node[12+12*i:]
		if level == 0 {
			l := int64(le.Uint16(e[4:]))
			if l > 32768 {
				l -= 32768 // an unwritten extent: allocated, reads as zeros
			}
			p := int64(le.Uint16(e[6:]))<<32 | int64(le.Uint32(e[8:]))
			if p == 0 || p+l > s.blocks {
				return out, errBadMap
			}
			out = append(out, extent{lblk: int64(le.Uint32(e)), pblk: p, len: l})
			continue
		}
		child := int64(le.Uint16(e[8:]))<<32 | int64(le.Uint32(e[4:]))
		if child == 0 || child >= s.blocks {
			return out, errBadMap
		}
		b, err := read(child)
		if err != nil {
			return out, err
		}
		sub, err := extentTree(b, read, s, depth+1)
		out = append(out, sub...)
		if err != nil {
			return out, err
		}
	}
	return out, nil
}

func blockMap(in *inode, read blockReader, s *superblock) ([]extent, error) {
	le := binary.LittleEndian
	per := s.blockSize / 4
	need := (in.size + s.blockSize - 1) / s.blockSize
	var out []extent
	lblk := int64(0)
	add := func(p int64) {
		if n := len(out); n > 0 && out[n-1].pblk+out[n-1].len == p && out[n-1].lblk+out[n-1].len == lblk {
			out[n-1].len++
		} else {
			out = append(out, extent{lblk: lblk, pblk: p, len: 1})
		}
	}
	var walk func(blk int64, level int) error
	walk = func(blk int64, level int) error {
		if blk == 0 || blk >= s.blocks {
			lblk += pow(per, level)
			return nil
		}
		if level == 0 {
			add(blk)
			lblk++
			return nil
		}
		b, err := read(blk)
		if err != nil {
			return err
		}
		for i := int64(0); i < per && lblk < need; i++ {
			if err := walk(int64(le.Uint32(b[4*i:])), level-1); err != nil {
				return err
			}
		}
		return nil
	}
	for i := 0; i < 12 && lblk < need; i++ {
		walk(int64(le.Uint32(in.block[4*i:])), 0)
	}
	for level, i := 1, 12; i < 15 && lblk < need; level, i = level+1, i+1 {
		if err := walk(int64(le.Uint32(in.block[4*i:])), level); err != nil {
			return out, err
		}
	}
	return out, nil
}

func pow(b int64, e int) int64 {
	r := int64(1)
	for ; e > 0; e-- {
		r *= b
	}
	return r
}
