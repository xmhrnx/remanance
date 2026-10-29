package ext4

import (
	"encoding/binary"
	"io"
)

type group struct {
	blockBitmap int64
	inodeTable  int64
	flags       uint16
}

const bgBlockUninit = 0x0002

func readGroups(r io.ReaderAt, s *superblock) ([]group, error) {
	n := s.groups()
	ds := s.descSize
	gdtBlock := s.firstDataBlock + 1
	out := make([]group, n)
	perBlock := s.blockSize / ds
	buf := make([]byte, s.blockSize)
	cur := int64(-1)
	for i := int64(0); i < n; i++ {
		blk := gdtBlock + i/perBlock
		if s.incompat&incompatMetaBG != 0 {
			// With meta_bg each run of groups keeps its descriptors in
			// the first group of the run.
			metaGroup := i / perBlock
			blk = metaGroup*perBlock*s.blocksPerGroup + s.firstDataBlock
			if hasSuperBackup(metaGroup * perBlock) {
				blk++
			}
		}
		if blk != cur {
			if _, err := r.ReadAt(buf, blk*s.blockSize); err != nil && err != io.EOF {
				return out, err
			}
			cur = blk
		}
		d := buf[(i%perBlock)*ds:]
		g := group{
			blockBitmap: int64(binary.LittleEndian.Uint32(d[0:])),
			inodeTable:  int64(binary.LittleEndian.Uint32(d[8:])),
			flags:       binary.LittleEndian.Uint16(d[18:]),
		}
		if ds >= 64 {
			g.blockBitmap |= int64(binary.LittleEndian.Uint32(d[0x20:])) << 32
			g.inodeTable |= int64(binary.LittleEndian.Uint32(d[0x28:])) << 32
		}
		out[i] = g
	}
	return out, nil
}

// hasSuperBackup tells whether a group holds a superblock copy under the
// sparse_super rule: groups 0, 1 and powers of 3, 5 and 7.
func hasSuperBackup(g int64) bool {
	if g <= 1 {
		return true
	}
	for _, base := range []int64{3, 5, 7} {
		p := base
		for p < g {
			p *= base
		}
		if p == g {
			return true
		}
	}
	return false
}
