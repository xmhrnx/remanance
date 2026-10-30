package sqlite

import (
	"encoding/binary"
	"math"
	"unicode/utf16"
)

// varint reads SQLite's big-endian variable-length integer: up to nine
// bytes, seven bits each but the ninth, which gives all eight.
func varint(b []byte) (v uint64, n int) {
	for i := 0; i < 9 && i < len(b); i++ {
		if i == 8 {
			return v<<8 | uint64(b[i]), 9
		}
		v = v<<7 | uint64(b[i]&0x7f)
		if b[i] < 0x80 {
			return v, i + 1
		}
	}
	return v, 0
}

// record decodes a row's payload into its values: nil, int64, float64,
// string or []byte.
func (db *DB) record(p []byte) []any {
	hl, n := varint(p)
	if n == 0 || hl > uint64(len(p)) || hl < uint64(n) {
		return nil
	}
	var types []uint64
	for i := n; i < int(hl); {
		t, k := varint(p[i:int(hl)])
		if k == 0 {
			break
		}
		types = append(types, t)
		i += k
	}
	body := p[hl:]
	out := make([]any, len(types))
	for i, t := range types {
		size := serialSize(t)
		if size > len(body) {
			// A cut-short record: what's missing reads as NULL.
			break
		}
		out[i] = db.value(t, body[:size])
		body = body[size:]
	}
	return out
}

// serialSize is how many bytes a value of serial type t takes.
func serialSize(t uint64) int {
	switch {
	case t <= 4:
		return []int{0, 1, 2, 3, 4}[t]
	case t == 5:
		return 6
	case t == 6 || t == 7:
		return 8
	case t < 12:
		return 0
	case t > math.MaxInt32:
		return math.MaxInt32
	}
	return int(t-12) / 2
}

func (db *DB) value(t uint64, b []byte) any {
	switch {
	case t == 0:
		return nil
	case t <= 6:
		// Big-endian two's complement, sign-extended from its width.
		var v int64
		if len(b) > 0 && b[0]&0x80 != 0 {
			v = -1
		}
		for _, c := range b {
			v = v<<8 | int64(c)
		}
		return v
	case t == 7:
		return math.Float64frombits(binary.BigEndian.Uint64(b))
	case t == 8:
		return int64(0)
	case t == 9:
		return int64(1)
	case t >= 12 && t%2 == 0:
		return append([]byte(nil), b...)
	case t >= 13:
		return db.text(b)
	}
	return nil
}

func (db *DB) text(b []byte) string {
	if db.enc == encUTF8 {
		return string(b)
	}
	u := make([]uint16, len(b)/2)
	for i := range u {
		if db.enc == encUTF16LE {
			u[i] = binary.LittleEndian.Uint16(b[2*i:])
		} else {
			u[i] = binary.BigEndian.Uint16(b[2*i:])
		}
	}
	return string(utf16.Decode(u))
}
