package plist

import (
	"encoding/binary"
	"fmt"
	"math"
	"time"
	"unicode/utf16"
)

// appleEpoch is when the dates in property lists count from.
var appleEpoch = time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)

// binaryList is a "bplist00" being decoded.
type binaryList struct {
	data    []byte
	offsets []uint64
	refSize int
	depth   int
}

func parseBinary(data []byte) (v any, err error) {
	if len(data) < 8+32 {
		return nil, ErrFormat
	}
	t := data[len(data)-32:]
	offSize, refSize := int(t[6]), int(t[7])
	count, top, table := binary.BigEndian.Uint64(t[8:]), binary.BigEndian.Uint64(t[16:]), binary.BigEndian.Uint64(t[24:])
	if offSize < 1 || offSize > 8 || refSize < 1 || refSize > 8 || count > uint64(len(data)) ||
		table > uint64(len(data)) || table+count*uint64(offSize) > uint64(len(data)) || top >= count {
		return nil, ErrFormat
	}
	b := &binaryList{data: data, refSize: refSize, offsets: make([]uint64, count)}
	for i := range b.offsets {
		b.offsets[i] = uintN(data[table+uint64(i*offSize):], offSize)
	}
	defer func() {
		// Damaged lists can index anywhere; out-of-range reads end up here.
		if r := recover(); r != nil {
			v, err = nil, fmt.Errorf("plist: damaged binary list (%v)", r)
		}
	}()
	return b.object(top)
}

func uintN(b []byte, n int) uint64 {
	var v uint64
	for i := range n {
		v = v<<8 | uint64(b[i])
	}
	return v
}

// object decodes object number i.
func (b *binaryList) object(i uint64) (any, error) {
	if i >= uint64(len(b.offsets)) {
		return nil, ErrFormat
	}
	b.depth++
	defer func() { b.depth-- }()
	if b.depth > 64 {
		return nil, fmt.Errorf("plist: nested too deeply")
	}
	p := b.offsets[i]
	marker := b.data[p]
	kind, n := marker>>4, uint64(marker&0x0f)
	p++
	switch kind {
	case 0x0:
		switch marker {
		case 0x08:
			return false, nil
		case 0x09:
			return true, nil
		}
		return nil, nil
	case 0x1:
		size := 1 << n
		if size == 16 {
			// 128-bit integers keep their value in the low 8 bytes.
			return int64(binary.BigEndian.Uint64(b.data[p+8:])), nil
		}
		// Shorter ones are unsigned, 8-byte ones signed: the conversion
		// gets both right.
		return int64(uintN(b.data[p:], size)), nil
	case 0x2:
		if n == 2 {
			return float64(math.Float32frombits(binary.BigEndian.Uint32(b.data[p:]))), nil
		}
		return math.Float64frombits(binary.BigEndian.Uint64(b.data[p:])), nil
	case 0x3:
		secs := math.Float64frombits(binary.BigEndian.Uint64(b.data[p:]))
		return appleEpoch.Add(time.Duration(secs * float64(time.Second))), nil
	case 0x8:
		return UID(uintN(b.data[p:], int(n)+1)), nil
	}
	// The rest carry a length, extended into an integer when it is 15.
	if n == 0xf {
		ext := b.data[p]
		size := 1 << (ext & 0x0f)
		n = uintN(b.data[p+1:], size)
		p += 1 + uint64(size)
	}
	switch kind {
	case 0x4:
		return append([]byte(nil), b.data[p:p+n]...), nil
	case 0x5:
		return string(b.data[p : p+n]), nil
	case 0x6:
		u := make([]uint16, n)
		for j := range u {
			u[j] = binary.BigEndian.Uint16(b.data[p+uint64(2*j):])
		}
		return string(utf16.Decode(u)), nil
	case 0xa, 0xc:
		out := make([]any, n)
		for j := range out {
			v, err := b.object(uintN(b.data[p+uint64(j*b.refSize):], b.refSize))
			if err != nil {
				return nil, err
			}
			out[j] = v
		}
		return out, nil
	case 0xd:
		out := make(map[string]any, n)
		for j := uint64(0); j < n; j++ {
			k, err := b.object(uintN(b.data[p+j*uint64(b.refSize):], b.refSize))
			if err != nil {
				return nil, err
			}
			v, err := b.object(uintN(b.data[p+(n+j)*uint64(b.refSize):], b.refSize))
			if err != nil {
				return nil, err
			}
			key, _ := k.(string)
			out[key] = v
		}
		return out, nil
	}
	return nil, fmt.Errorf("plist: unknown object type %#x", marker)
}
