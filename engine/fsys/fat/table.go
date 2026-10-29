package fat

import (
	"encoding/binary"
	"io"
)

// table is the file allocation table: entry n is the cluster after n in
// its chain, 0 when free.
type table struct {
	bits    int
	raw     []byte
	entries uint32
}

func readTable(r io.ReaderAt, b *bpb) (*table, error) {
	size := b.fatSectors * b.bytesPerSector
	t := &table{bits: b.bits, raw: make([]byte, size), entries: b.clusters + 2}
	if _, err := r.ReadAt(t.raw, b.fatOffset); err != nil && err != io.EOF {
		// The first copy is unreadable; try the others.
		for i := int64(1); i < b.fats; i++ {
			if _, err2 := r.ReadAt(t.raw, b.fatOffset+i*size); err2 == nil {
				return t, nil
			}
		}
		return t, err
	}
	return t, nil
}

func (t *table) get(c uint32) uint32 {
	if c >= t.entries {
		return 0
	}
	switch t.bits {
	case 32:
		if int(c)*4+4 > len(t.raw) {
			return 0
		}
		return binary.LittleEndian.Uint32(t.raw[c*4:]) & 0x0FFFFFFF
	case 16:
		if int(c)*2+2 > len(t.raw) {
			return 0
		}
		return uint32(binary.LittleEndian.Uint16(t.raw[c*2:]))
	default:
		i := c + c/2
		if int(i)+2 > len(t.raw) {
			return 0
		}
		v := uint32(binary.LittleEndian.Uint16(t.raw[i:]))
		if c&1 != 0 {
			return v >> 4
		}
		return v & 0x0FFF
	}
}

func (t *table) isEnd(v uint32) bool {
	switch t.bits {
	case 32:
		return v >= 0x0FFFFFF8
	case 16:
		return v >= 0xFFF8
	default:
		return v >= 0x0FF8
	}
}

func (t *table) used(c uint32) bool { return t.get(c) != 0 }

// chain follows the clusters of an existing file from first, stopping at
// the end mark, a free or bad entry, a loop, or max clusters.
func (t *table) chain(first uint32, max int64) []uint32 {
	var out []uint32
	seen := map[uint32]bool{}
	for c := first; c >= 2 && c < t.entries && int64(len(out)) < max; {
		if seen[c] {
			break
		}
		seen[c] = true
		out = append(out, c)
		next := t.get(c)
		if next == 0 || t.isEnd(next) || next == 0x0FFFFFF7 || next == 0xFFF7 || next == 0xFF7 {
			break
		}
		c = next
	}
	return out
}
