package fat

import (
	"encoding/binary"
	"strings"
	"time"
)

type lfnPart struct {
	seq      int
	checksum byte
	chars    []uint16
}

const (
	attrReadOnly  = 0x01
	attrHidden    = 0x02
	attrSystem    = 0x04
	attrVolumeID  = 0x08
	attrDirectory = 0x10
	attrLFN       = 0x0F

	entryDeleted = 0xE5
)

// dirent is a short directory entry together with the long name that
// preceded it.
type dirent struct {
	name     string
	deleted  bool
	isDir    bool
	cluster  uint32
	size     int64
	created  time.Time
	modified time.Time
	accessed time.Time
	offset   int64 // where the entry is in the volume, unique per file
}

func dosTime(date, tm uint16) time.Time {
	if date == 0 {
		return time.Time{}
	}
	return time.Date(1980+int(date>>9), time.Month(date>>5&15), int(date&31),
		int(tm>>11), int(tm>>5&63), int(tm&31)*2, 0, time.Local)
}

// shortName renders an 8.3 name, applying the lower-case flags Windows
// keeps in byte 12.
func shortName(e []byte) string {
	base := strings.TrimRight(string(e[0:8]), " ")
	ext := strings.TrimRight(string(e[8:11]), " ")
	if e[12]&0x08 != 0 {
		base = strings.ToLower(base)
	}
	if e[12]&0x10 != 0 {
		ext = strings.ToLower(ext)
	}
	if ext == "" {
		return base
	}
	return base + "." + ext
}

// plausible rejects 32 bytes that aren't a short directory entry: names
// with control characters or bytes FAT forbids, impossible dates.
func plausible(e []byte, deleted bool) bool {
	start := 0
	if deleted {
		start = 1
	}
	for i := start; i < 11; i++ {
		c := e[i]
		if c < 0x20 || strings.IndexByte("\"*+,/:;<=>?\\[]|", c) >= 0 {
			return false
		}
	}
	if e[11]&0xC0 != 0 {
		return false
	}
	if d := binary.LittleEndian.Uint16(e[24:]); d != 0 && (d&31 == 0 || d>>5&15 == 0 || d>>5&15 > 12) {
		return false
	}
	return true
}
