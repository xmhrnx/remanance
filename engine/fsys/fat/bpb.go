// Package fat reads FAT12, FAT16 and FAT32 volumes: their files, the
// directory entries of deleted ones, and folders a quick format orphaned.
package fat

import (
	"encoding/binary"
	"errors"
)

type bpb struct {
	bytesPerSector int64
	clusterSize    int64
	reserved       int64
	fats           int64
	rootEntries    int64
	totalSectors   int64
	fatSectors     int64
	rootCluster    uint32 // FAT32
	bits           int    // 12, 16 or 32
	label          string

	fatOffset  int64
	rootOffset int64 // FAT12/16 fixed root directory
	rootSize   int64
	dataOffset int64
	clusters   uint32 // data clusters; valid numbers are 2..clusters+1
}

var errNotFAT = errors.New("fat: not a FAT boot sector")

func parseBPB(s []byte) (bpb, error) {
	if len(s) < 512 || s[510] != 0x55 || s[511] != 0xAA {
		return bpb{}, errNotFAT
	}
	b := bpb{
		bytesPerSector: int64(binary.LittleEndian.Uint16(s[11:])),
		reserved:       int64(binary.LittleEndian.Uint16(s[14:])),
		fats:           int64(s[16]),
		rootEntries:    int64(binary.LittleEndian.Uint16(s[17:])),
		totalSectors:   int64(binary.LittleEndian.Uint16(s[19:])),
		fatSectors:     int64(binary.LittleEndian.Uint16(s[22:])),
	}
	spc := int64(s[13])
	if b.bytesPerSector < 512 || b.bytesPerSector > 4096 || b.bytesPerSector&(b.bytesPerSector-1) != 0 ||
		spc == 0 || spc&(spc-1) != 0 || b.reserved == 0 || b.fats == 0 || b.fats > 4 {
		return bpb{}, errNotFAT
	}
	b.clusterSize = b.bytesPerSector * spc
	if b.totalSectors == 0 {
		b.totalSectors = int64(binary.LittleEndian.Uint32(s[32:]))
	}
	labelAt := 43
	if b.fatSectors == 0 {
		b.fatSectors = int64(binary.LittleEndian.Uint32(s[36:]))
		b.rootCluster = binary.LittleEndian.Uint32(s[44:])
		labelAt = 71
	}
	if b.fatSectors == 0 || b.totalSectors == 0 {
		return bpb{}, errNotFAT
	}
	if s[labelAt-5] == 0x29 {
		b.label = trimSpaces(s[labelAt : labelAt+11])
	}
	bps := b.bytesPerSector
	b.fatOffset = b.reserved * bps
	b.rootOffset = b.fatOffset + b.fats*b.fatSectors*bps
	b.rootSize = b.rootEntries * 32
	rootSectors := (b.rootSize + bps - 1) / bps
	b.dataOffset = b.rootOffset + rootSectors*bps
	dataSectors := b.totalSectors - b.dataOffset/bps
	if dataSectors <= 0 {
		return bpb{}, errNotFAT
	}
	b.clusters = uint32(dataSectors / spc)
	switch {
	case b.clusters < 4085:
		b.bits = 12
	case b.clusters < 65525:
		b.bits = 16
	default:
		b.bits = 32
	}
	if b.bits == 32 && b.rootEntries != 0 {
		b.bits = 16 // a FAT16 volume with an unusually high cluster count
	}
	if b.bits != 32 {
		b.rootCluster = 0
	}
	return b, nil
}

func trimSpaces(b []byte) string {
	end := len(b)
	for end > 0 && (b[end-1] == ' ' || b[end-1] == 0) {
		end--
	}
	s := string(b[:end])
	if s == "NO NAME" {
		return ""
	}
	return s
}

// clusterOffset is where cluster c starts in the volume.
func (b *bpb) clusterOffset(c uint32) int64 {
	return b.dataOffset + int64(c-2)*b.clusterSize
}

func (b *bpb) validCluster(c uint32) bool { return c >= 2 && c < b.clusters+2 }
