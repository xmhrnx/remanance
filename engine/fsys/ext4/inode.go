package ext4

import (
	"encoding/binary"
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

// extent maps len blocks of a file from logical block lblk to physical
// block pblk.
type extent struct {
	lblk, pblk, len int64
}

func (i *inode) isDir() bool { return i.mode&modeTypeMask == modeDir }

func (i *inode) isFile() bool { return i.mode&modeTypeMask == modeFile }

func (i *inode) live() bool { return i.links > 0 && i.dtime == 0 }

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
