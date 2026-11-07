package plist

import (
	"bytes"
	"encoding/binary"
	"math"
)

// marker is an object's type and count, the count after it when it
// doesn't fit in four bits.
func marker(kind byte, n int) []byte {
	if n < 15 {
		return []byte{kind<<4 | byte(n)}
	}
	return append([]byte{kind<<4 | 0xf}, intObject(int64(n))...)
}

func intObject(n int64) []byte {
	switch {
	case n >= 0 && n <= math.MaxUint8:
		return []byte{0x10, byte(n)}
	case n >= 0 && n <= math.MaxUint16:
		return binary.BigEndian.AppendUint16([]byte{0x11}, uint16(n))
	case n >= 0 && n <= math.MaxUint32:
		return binary.BigEndian.AppendUint32([]byte{0x12}, uint32(n))
	}
	return binary.BigEndian.AppendUint64([]byte{0x13}, uint64(n))
}

func sizeFor(n uint64) int {
	switch {
	case n <= math.MaxUint8:
		return 1
	case n <= math.MaxUint16:
		return 2
	case n <= math.MaxUint32:
		return 4
	}
	return 8
}

func putN(b *bytes.Buffer, v uint64, n int) {
	for i := n - 1; i >= 0; i-- {
		b.WriteByte(byte(v >> (8 * i)))
	}
}
