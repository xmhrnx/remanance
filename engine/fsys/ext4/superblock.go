// Package ext4 reads ext2, ext3 and ext4 volumes. ext4 erases a deleted
// file's extents and size from its inode, so deleted files are rebuilt
// from older copies of their inodes kept in the journal.
package ext4

import (
	"encoding/binary"
	"errors"
)

const (
	incompatFiletype   = 0x0002
	incompatExtents    = 0x0040
	incompat64bit      = 0x0080
	incompatMetaBG     = 0x0010
	incompatFlexBG     = 0x0200
	incompatInlineData = 0x8000
	compatHasJournal   = 0x0004
)

type superblock struct {
	inodes         uint32
	blocks         int64
	firstDataBlock int64
	blockSize      int64
	blocksPerGroup int64
	inodesPerGroup uint32
	inodeSize      int64
	firstIno       uint32
	compat         uint32
	incompat       uint32
	journalInode   uint32
	descSize       int64
	label          string
}

var errNotExt = errors.New("ext4: no superblock")

func parseSuperblock(b []byte) (superblock, error) {
	if len(b) < 1024 || binary.LittleEndian.Uint16(b[56:]) != 0xEF53 {
		return superblock{}, errNotExt
	}
	le := binary.LittleEndian
	s := superblock{
		inodes:         le.Uint32(b[0:]),
		blocks:         int64(le.Uint32(b[4:])),
		firstDataBlock: int64(le.Uint32(b[20:])),
		blockSize:      1024 << le.Uint32(b[24:]),
		blocksPerGroup: int64(le.Uint32(b[32:])),
		inodesPerGroup: le.Uint32(b[40:]),
		inodeSize:      128,
		firstIno:       11,
		compat:         le.Uint32(b[92:]),
		incompat:       le.Uint32(b[96:]),
		journalInode:   le.Uint32(b[0xE0:]),
		descSize:       32,
	}
	if le.Uint32(b[76:]) >= 1 { // dynamic revision
		s.firstIno = le.Uint32(b[84:])
		s.inodeSize = int64(le.Uint16(b[88:]))
	}
	if s.incompat&incompat64bit != 0 {
		s.blocks |= int64(le.Uint32(b[0x150:])) << 32
		if ds := int64(le.Uint16(b[0xFE:])); ds >= 64 {
			s.descSize = ds
		}
	}
	end := 120
	for end < 136 && b[end] != 0 {
		end++
	}
	s.label = string(b[120:end])
	if s.blockSize > 65536 || s.blocksPerGroup == 0 || s.inodesPerGroup == 0 || s.inodeSize < 128 || s.inodeSize > s.blockSize {
		return superblock{}, errors.New("ext4: implausible superblock")
	}
	return s, nil
}

func (s *superblock) groups() int64 {
	return (s.blocks - s.firstDataBlock + s.blocksPerGroup - 1) / s.blocksPerGroup
}

func (s *superblock) fsName() string {
	switch {
	case s.incompat&(incompatExtents|incompatFlexBG) != 0:
		return "ext4"
	case s.compat&compatHasJournal != 0:
		return "ext3"
	}
	return "ext2"
}
