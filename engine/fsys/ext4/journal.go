package ext4

import (
	"encoding/binary"
	"sort"
)

const (
	jbdMagic      = 0xC03B3998
	jbdDescriptor = 1
	jbdCommit     = 2
	jbdSuperV1    = 3
	jbdSuperV2    = 4
	jbdRevoke     = 5

	jbdFeature64     = 0x2
	jbdFeatureCsumV2 = 0x8
	jbdFeatureCsumV3 = 0x10

	tagEscape   = 0x1
	tagSameUUID = 0x2
	tagLast     = 0x8
)

// journalCopy is one logged copy of a file system block.
type journalCopy struct {
	seq     uint32
	jblock  int64 // physical block of the copy
	escaped bool
}

// journal indexes the copies of metadata blocks still in the jbd2 log.
// The log is a ring that's only overwritten as new transactions come in,
// so it usually still holds inode table blocks from before a deletion.
type journal struct {
	copies map[int64][]journalCopy // file system block -> copies, oldest first
}

// readJournal scans every block of the log, not just the live part, to
// find as many old transactions as it can.
func (vol *volume) readJournal() *journal {
	s := &vol.s
	if s.compat&compatHasJournal == 0 || s.journalInode == 0 {
		return nil
	}
	jin, err := vol.readInode(s.journalInode)
	if err != nil {
		return nil
	}
	ext, err := jin.mapping(vol.readBlock, s)
	if err != nil && len(ext) == 0 {
		return nil
	}
	phys := func(l int64) int64 {
		for _, e := range ext {
			if l >= e.lblk && l < e.lblk+e.len {
				return e.pblk + l - e.lblk
			}
		}
		return -1
	}
	be := binary.BigEndian
	sb, err := vol.readBlock(phys(0))
	if err != nil || be.Uint32(sb) != jbdMagic {
		return nil
	}
	if t := be.Uint32(sb[4:]); t != jbdSuperV1 && t != jbdSuperV2 {
		return nil
	}
	maxLen := int64(be.Uint32(sb[16:]))
	first := int64(be.Uint32(sb[20:]))
	var incompat uint32
	if be.Uint32(sb[4:]) == jbdSuperV2 {
		incompat = be.Uint32(sb[40:])
	}
	tagSize := 8
	switch {
	case incompat&jbdFeatureCsumV3 != 0:
		tagSize = 16
	case incompat&jbdFeature64 != 0:
		tagSize = 12
	}
	tail := 0
	if incompat&(jbdFeatureCsumV2|jbdFeatureCsumV3) != 0 {
		tail = 4
	}

	j := &journal{copies: map[int64][]journalCopy{}}
	for l := first; l < maxLen; l++ {
		b, err := vol.readBlock(phys(l))
		if err != nil || be.Uint32(b) != jbdMagic || be.Uint32(b[4:]) != jbdDescriptor {
			continue
		}
		seq := be.Uint32(b[8:])
		data := l + 1
		for off := 12; off+tagSize <= len(b)-tail; {
			blk := int64(be.Uint32(b[off:]))
			var flags uint32
			if tagSize == 16 {
				flags = be.Uint32(b[off+4:])
				blk |= int64(be.Uint32(b[off+8:])) << 32
			} else {
				flags = uint32(be.Uint16(b[off+6:]))
				if tagSize == 12 {
					blk |= int64(be.Uint32(b[off+8:])) << 32
				}
			}
			off += tagSize
			if flags&tagSameUUID == 0 {
				off += 16
			}
			if data >= maxLen {
				data = first // the transaction wrapped around the ring
			}
			if p := phys(data); p >= 0 && blk < s.blocks {
				j.copies[blk] = append(j.copies[blk], journalCopy{seq: seq, jblock: p, escaped: flags&tagEscape != 0})
			}
			data++
			if flags&tagLast != 0 {
				break
			}
		}
		l = data - 1
	}
	for _, c := range j.copies {
		sort.Slice(c, func(a, b int) bool { return c[a].seq < c[b].seq })
	}
	return j
}

// versions returns every logged copy of a file system block, newest
// first.
func (vol *volume) versions(blk int64) [][]byte {
	if vol.j == nil {
		return nil
	}
	cs := vol.j.copies[blk]
	out := make([][]byte, 0, len(cs))
	for i := len(cs) - 1; i >= 0; i-- {
		b, err := vol.readBlock(cs[i].jblock)
		if err != nil {
			continue
		}
		if cs[i].escaped {
			b = append([]byte(nil), b...)
			binary.BigEndian.PutUint32(b, jbdMagic)
		}
		out = append(out, b)
	}
	return out
}
