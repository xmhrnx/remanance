package exfat

import (
	"encoding/binary"
	"time"
	"unicode/utf16"
)

const (
	typeBitmap = 0x01
	typeLabel  = 0x03
	typeFile   = 0x05
	typeStream = 0x40
	typeName   = 0x41
	inUse      = 0x80

	attrDirectory = 0x10
	flagNoChain   = 0x02
)

// fileSet is a file directory entry with its stream and name entries.
type fileSet struct {
	deleted  bool
	isDir    bool
	name     string
	noChain  bool
	cluster  uint32
	size     int64
	created  time.Time
	modified time.Time
	accessed time.Time
	offset   int64
}

func timestamp(v uint32, centis byte, utcOff byte) time.Time {
	if v == 0 {
		return time.Time{}
	}
	t := time.Date(1980+int(v>>25), time.Month(v>>21&15), int(v>>16&31),
		int(v>>11&31), int(v>>5&63), int(v&31)*2, int(centis)*10*int(time.Millisecond), time.UTC)
	if utcOff&0x80 != 0 {
		// The offset is in 15-minute steps, signed in seven bits.
		q := int(int8(utcOff<<1) >> 1)
		return t.Add(-time.Duration(q) * 15 * time.Minute)
	}
	return t.Local()
}

// parseDir splits a directory's bytes into file sets, deleted ones too.
// It also reports the allocation bitmap's location, which the root
// directory holds, and the volume label.
func parseDir(raw []byte, base int64) (sets []fileSet, bitmapCluster uint32, bitmapLen int64, label string) {
	for off := 0; off+32 <= len(raw); off += 32 {
		e := raw[off : off+32]
		typ := e[0]
		if typ == 0 {
			break
		}
		code := typ &^ inUse
		switch {
		case typ == inUse|typeBitmap:
			bitmapCluster = binary.LittleEndian.Uint32(e[20:])
			bitmapLen = int64(binary.LittleEndian.Uint64(e[24:]))
		case typ == inUse|typeLabel:
			n := min(int(e[1]), 11)
			label = utf16Str(e[2 : 2+2*n])
		case code == typeFile:
			secondaries := int(e[1])
			if secondaries < 2 || secondaries > 18 || off+32*(secondaries+1) > len(raw) {
				continue
			}
			set, ok := parseSet(raw[off:off+32*(secondaries+1)], typ&inUse == 0)
			if !ok {
				continue
			}
			set.offset = base + int64(off)
			sets = append(sets, set)
			off += 32 * secondaries
		}
	}
	return
}

func parseSet(b []byte, deleted bool) (fileSet, bool) {
	stream := b[32:64]
	want := byte(typeStream)
	if !deleted {
		want |= inUse
	}
	if stream[0] != want {
		return fileSet{}, false
	}
	f := fileSet{
		deleted:  deleted,
		isDir:    binary.LittleEndian.Uint16(b[4:])&attrDirectory != 0,
		created:  timestamp(binary.LittleEndian.Uint32(b[8:]), b[20], b[22]),
		modified: timestamp(binary.LittleEndian.Uint32(b[12:]), b[21], b[23]),
		accessed: timestamp(binary.LittleEndian.Uint32(b[16:]), 0, b[24]),
		noChain:  stream[1]&flagNoChain != 0,
		cluster:  binary.LittleEndian.Uint32(stream[20:]),
		size:     int64(binary.LittleEndian.Uint64(stream[24:])),
	}
	nameLen := int(stream[3])
	var u []uint16
	for i := 64; i+32 <= len(b) && len(u) < nameLen; i += 32 {
		if b[i]&^inUse != typeName {
			break
		}
		for j := 2; j < 32 && len(u) < nameLen; j += 2 {
			u = append(u, binary.LittleEndian.Uint16(b[i+j:]))
		}
	}
	if len(u) == 0 {
		return fileSet{}, false
	}
	f.name = string(utf16.Decode(u))
	return f, true
}

func utf16Str(b []byte) string {
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(b[2*i:])
	}
	return string(utf16.Decode(u))
}
