package ntfs

import (
	"encoding/binary"
	"errors"
)

const (
	recordInUse     = 0x0001
	recordDirectory = 0x0002
)

// record is one parsed MFT record.
type record struct {
	number  int64
	seq     uint16
	flags   uint16
	baseRef int64 // base record number for extension records, else 0
	torn    bool  // a fixup didn't match: written partially
	attrs   []attribute
}

func (r *record) inUse() bool { return r.flags&recordInUse != 0 }
func (r *record) isDir() bool { return r.flags&recordDirectory != 0 }

// detach copies attribute values out of the buffer the record was parsed
// from.
func (r *record) detach() {
	for i := range r.attrs {
		r.attrs[i].value = append([]byte(nil), r.attrs[i].value...)
	}
}

var errNoRecord = errors.New("ntfs: not an MFT record")

// applyFixups restores the last two bytes of every sector of a multi
// sector structure (MFT record, index block) from its update sequence
// array. It reports false when a sector wasn't written with the rest.
func applyFixups(b []byte) bool {
	usaOff := int(binary.LittleEndian.Uint16(b[4:]))
	usaCount := int(binary.LittleEndian.Uint16(b[6:]))
	if usaCount < 2 || usaOff+2*usaCount > len(b) {
		return false
	}
	stride := len(b) / (usaCount - 1)
	if stride < 256 {
		return false
	}
	usn := b[usaOff : usaOff+2]
	ok := true
	for i := 1; i < usaCount; i++ {
		pos := i*stride - 2
		if pos+2 > len(b) {
			break
		}
		if b[pos] != usn[0] || b[pos+1] != usn[1] {
			ok = false
		}
		b[pos], b[pos+1] = b[usaOff+2*i], b[usaOff+2*i+1]
	}
	return ok
}

// parseRecord parses an MFT record in b, modifying b to undo fixups.
func parseRecord(b []byte, number int64) (*record, error) {
	if len(b) < 48 || string(b[:4]) != "FILE" {
		return nil, errNoRecord
	}
	r := &record{number: number}
	r.torn = !applyFixups(b)
	r.seq = binary.LittleEndian.Uint16(b[16:])
	r.flags = binary.LittleEndian.Uint16(b[22:])
	r.baseRef = int64(binary.LittleEndian.Uint64(b[32:]) & 0xFFFFFFFFFFFF)
	off := int(binary.LittleEndian.Uint16(b[20:]))
	used := int(binary.LittleEndian.Uint32(b[24:]))
	if used > len(b) || used == 0 {
		used = len(b)
	}
	for off+16 <= used {
		a, n, ok := parseAttribute(b[off:used])
		if !ok {
			break
		}
		r.attrs = append(r.attrs, a)
		off += n
	}
	return r, nil
}
