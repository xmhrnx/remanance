package ntfs

import "encoding/binary"

const namespaceDOS = 2

func parseFileName(v []byte) (fileName, bool) {
	if len(v) < 66 {
		return fileName{}, false
	}
	ref := binary.LittleEndian.Uint64(v)
	n := int(v[64])
	if 66+2*n > len(v) {
		return fileName{}, false
	}
	return fileName{
		parent:    int64(ref & 0xFFFFFFFFFFFF),
		parentSeq: uint16(ref >> 48),
		created:   filetime(binary.LittleEndian.Uint64(v[8:])),
		modified:  filetime(binary.LittleEndian.Uint64(v[16:])),
		accessed:  filetime(binary.LittleEndian.Uint64(v[32:])),
		size:      int64(binary.LittleEndian.Uint64(v[48:])),
		namespace: v[65],
		name:      utf16String(v[66 : 66+2*n]),
	}, true
}

func parseStandardInfo(v []byte) (standardInfo, bool) {
	if len(v) < 32 {
		return standardInfo{}, false
	}
	return standardInfo{
		created:  filetime(binary.LittleEndian.Uint64(v[0:])),
		modified: filetime(binary.LittleEndian.Uint64(v[8:])),
		accessed: filetime(binary.LittleEndian.Uint64(v[24:])),
	}, true
}
