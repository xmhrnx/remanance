package ntfs

import (
	"encoding/binary"
	"time"
	"unicode/utf16"
)

const (
	attrStandardInfo = 0x10
	attrList         = 0x20
	attrFileName     = 0x30
	attrData         = 0x80
	attrIndexRoot    = 0x90
	attrBitmap       = 0xB0
	attrEnd          = 0xFFFFFFFF

	flagCompressed = 0x0001
	flagEncrypted  = 0x4000
	flagSparse     = 0x8000
)

// attribute is one attribute of a record. Resident attributes carry their
// value; non-resident ones their run list.
type attribute struct {
	typ         uint32
	name        string
	flags       uint16
	resident    bool
	value       []byte // resident value
	startVCN    int64
	lastVCN     int64
	runs        []run
	allocSize   int64
	dataSize    int64
	initSize    int64
	compUnitLog uint16
}

func parseAttribute(b []byte) (attribute, int, bool) {
	typ := binary.LittleEndian.Uint32(b)
	if typ == attrEnd || len(b) < 16 {
		return attribute{}, 0, false
	}
	length := int(binary.LittleEndian.Uint32(b[4:]))
	if length < 16 || length > len(b) || length%8 != 0 {
		return attribute{}, 0, false
	}
	b = b[:length]
	a := attribute{typ: typ, resident: b[8] == 0, flags: binary.LittleEndian.Uint16(b[12:])}
	if nameLen := int(b[9]); nameLen > 0 {
		nameOff := int(binary.LittleEndian.Uint16(b[10:]))
		if nameOff+2*nameLen <= length {
			a.name = utf16String(b[nameOff : nameOff+2*nameLen])
		}
	}
	if a.resident {
		if length < 24 {
			return attribute{}, 0, false
		}
		vlen := int(binary.LittleEndian.Uint32(b[16:]))
		voff := int(binary.LittleEndian.Uint16(b[20:]))
		if voff+vlen > length {
			return attribute{}, 0, false
		}
		a.value = b[voff : voff+vlen]
		a.dataSize = int64(vlen)
		return a, length, true
	}
	if length < 64 {
		return attribute{}, 0, false
	}
	a.startVCN = int64(binary.LittleEndian.Uint64(b[16:]))
	a.lastVCN = int64(binary.LittleEndian.Uint64(b[24:]))
	runsOff := int(binary.LittleEndian.Uint16(b[32:]))
	a.compUnitLog = binary.LittleEndian.Uint16(b[34:])
	a.allocSize = int64(binary.LittleEndian.Uint64(b[40:]))
	a.dataSize = int64(binary.LittleEndian.Uint64(b[48:]))
	a.initSize = int64(binary.LittleEndian.Uint64(b[56:]))
	if runsOff < length {
		a.runs = decodeRuns(b[runsOff:])
	}
	return a, length, true
}

func utf16String(b []byte) string {
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(b[2*i:])
	}
	return string(utf16.Decode(u))
}

// filetime converts a Windows FILETIME (100 ns ticks since 1601).
func filetime(v uint64) time.Time {
	if v == 0 {
		return time.Time{}
	}
	const epochDelta = 116444736000000000
	if v < epochDelta {
		return time.Time{}
	}
	return time.Unix(0, int64(v-epochDelta)*100).UTC()
}

// fileName is a $FILE_NAME attribute's value.
type fileName struct {
	parent    int64
	parentSeq uint16
	created   time.Time
	modified  time.Time
	accessed  time.Time
	size      int64
	namespace byte
	name      string
}

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

// standardInfo holds the times of $STANDARD_INFORMATION, which, unlike
// $FILE_NAME's, follow every change to the file.
type standardInfo struct {
	created, modified, accessed time.Time
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
