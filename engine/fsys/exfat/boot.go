// Package exfat reads exFAT volumes (SD cards, camera cards, large USB
// drives), deleted entries included.
package exfat

import (
	"encoding/binary"
	"errors"
)

type boot struct {
	sector      int64
	clusterSize int64
	fatOffset   int64 // bytes
	fatLength   int64
	heapOffset  int64
	clusters    uint32
	rootCluster uint32
}

var errNotExFAT = errors.New("exfat: not an exFAT boot sector")

func parseBoot(s []byte) (boot, error) {
	if len(s) < 512 || string(s[3:11]) != "EXFAT   " {
		return boot{}, errNotExFAT
	}
	bShift, cShift := s[108], s[109]
	if bShift < 9 || bShift > 12 || int(bShift)+int(cShift) > 25 {
		return boot{}, errNotExFAT
	}
	b := boot{sector: 1 << bShift}
	b.clusterSize = b.sector << cShift
	b.fatOffset = int64(binary.LittleEndian.Uint32(s[80:])) * b.sector
	b.fatLength = int64(binary.LittleEndian.Uint32(s[84:])) * b.sector
	b.heapOffset = int64(binary.LittleEndian.Uint32(s[88:])) * b.sector
	b.clusters = binary.LittleEndian.Uint32(s[92:])
	b.rootCluster = binary.LittleEndian.Uint32(s[96:])
	if b.clusters == 0 || b.rootCluster < 2 {
		return boot{}, errNotExFAT
	}
	return b, nil
}

func (b *boot) clusterOffset(c uint32) int64 { return b.heapOffset + int64(c-2)*b.clusterSize }
func (b *boot) valid(c uint32) bool          { return c >= 2 && c < b.clusters+2 }
