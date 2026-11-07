package sqlite

import "encoding/binary"

// leaf reads the cells a table leaf page at p[h] still lists, then its
// free space. It reports false if p isn't one.
func (f *finder) leaf(p []byte, h int, where string) bool {
	if h+8 > len(p) || p[h] != leafTable {
		return false
	}
	cells := int(binary.BigEndian.Uint16(p[h+3:]))
	if h+8+2*cells > len(p) {
		return false
	}
	for i := range cells {
		c := int(binary.BigEndian.Uint16(p[h+8+2*i:]))
		if c < h+8 || c >= len(p) {
			continue
		}
		size, k := varint(p[c:])
		rowid, k2 := varint(p[c+k:])
		if k == 0 || k2 == 0 || size == 0 {
			continue
		}
		payload, err := f.db.payload(p, c+k+k2, int64(size))
		if err != nil {
			// The overflow pages are gone or reused: keep what the page
			// has.
			payload = p[c+k+k2 : min(len(p), c+k+k2+int(size))]
		}
		types, hl, ok := header(payload, 0, len(payload))
		if !ok {
			continue
		}
		if t := f.match(types, false); t != nil {
			f.add(t, int64(rowid), f.values(payload, hl, types), where)
		}
	}
	f.freeSpace(p, h, where)
	return true
}

// freeSpace carves the free space of a table leaf page at p[h]: the gap
// between its list of cells and their content, and its freeblocks.
func (f *finder) freeSpace(p []byte, h int, where string) {
	cells := int(binary.BigEndian.Uint16(p[h+3:]))
	start := h + 8 + 2*cells
	content := int(binary.BigEndian.Uint16(p[h+5:]))
	if content == 0 {
		content = 65536
	}
	if start < content && content <= len(p) {
		f.carve(p, start, content, where)
	}
	fb := int(binary.BigEndian.Uint16(p[h+1:]))
	for i := 0; fb != 0 && i < len(p)/4; i++ {
		if fb < start || fb+4 > len(p) {
			break
		}
		next, size := int(binary.BigEndian.Uint16(p[fb:])), int(binary.BigEndian.Uint16(p[fb+2:]))
		end := min(fb+size, len(p))
		from := fb + 4
		// The freeblock's own four bytes went over the start of the cell:
		// its lengths and rowid, and maybe the record header's length and
		// first type too.
		if r, ok := f.headless(p, from, end, where); ok {
			from = r
		}
		if from < end {
			f.carve(p, from, end, where)
		}
		if next != 0 && next <= fb {
			break
		}
		fb = next
	}
}

// headless finds a record whose header lost its first byte or two at
// p[from], and returns where it ends. Of the tables it could belong to,
// it takes the one whose row fills the most of the freeblock, which
// was the size of the cell.
func (f *finder) headless(p []byte, from, end int, where string) (int, bool) {
	var (
		best    *Table
		bestEnd int
		bestV   []any
	)
	for _, t := range f.tables {
		n := len(t.Columns)
		for lost := 1; lost <= 2; lost++ {
			want := n - (lost - 1)
			if lost == 2 && t.rowidCol != 0 || want < 1 {
				continue
			}
			var types []uint64
			at := from
			for len(types) < want && at < end {
				v, k := varint(p[at:end])
				if k == 0 {
					break
				}
				types = append(types, v)
				at += k
			}
			if len(types) != want {
				continue
			}
			if lost == 2 {
				types = append([]uint64{0}, types...) // the rowid's NULL
			}
			body := 0
			for _, ty := range types {
				body += serialSize(ty)
			}
			if at+body > end || at+body <= bestEnd || !t.fits(types, true) {
				continue
			}
			v := f.decode(p[at:at+body], types)
			if !plausible(v, types) {
				continue
			}
			best, bestEnd, bestV = t, at+body, v
		}
	}
	if best == nil {
		return 0, false
	}
	f.add(best, 0, bestV, where)
	return bestEnd, true
}
