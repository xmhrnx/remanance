// Package ntfs reads NTFS volumes, including the MFT records of deleted
// files, whose cluster runs NTFS leaves in place on deletion.
package ntfs

import (
	"encoding/binary"
	"errors"
)

type boot struct {
	bytesPerSector int64
	clusterSize    int64
	totalSectors   int64
	mftLCN         int64
	mftMirrLCN     int64
	recordSize     int64
	indexSize      int64
	serial         uint64
}

var errNotNTFS = errors.New("ntfs: not an NTFS boot sector")

// sizeField decodes the boot sector's "clusters per record" bytes: a
// positive count of clusters, or a negative power of two in bytes.
func sizeField(b byte, cluster int64) int64 {
	v := int8(b)
	if v > 0 {
		return int64(v) * cluster
	}
	return 1 << uint(-v)
}

func parseBoot(s []byte) (boot, error) {
	if len(s) < 512 || string(s[3:11]) != "NTFS    " {
		return boot{}, errNotNTFS
	}
	b := boot{bytesPerSector: int64(binary.LittleEndian.Uint16(s[11:]))}
	spc := int64(s[13])
	if spc > 0x80 {
		spc = 1 << (256 - spc)
	}
	if b.bytesPerSector < 256 || b.bytesPerSector > 4096 || spc == 0 {
		return boot{}, errNotNTFS
	}
	b.clusterSize = b.bytesPerSector * spc
	b.totalSectors = int64(binary.LittleEndian.Uint64(s[0x28:]))
	b.mftLCN = int64(binary.LittleEndian.Uint64(s[0x30:]))
	b.mftMirrLCN = int64(binary.LittleEndian.Uint64(s[0x38:]))
	b.recordSize = sizeField(s[0x40], b.clusterSize)
	b.indexSize = sizeField(s[0x44], b.clusterSize)
	b.serial = binary.LittleEndian.Uint64(s[0x48:])
	if b.recordSize < 256 || b.recordSize > 65536 {
		return boot{}, errors.New("ntfs: implausible MFT record size")
	}
	return b, nil
}
