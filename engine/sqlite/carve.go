package sqlite

import (
	"encoding/binary"
	"fmt"
)

// finder gathers the records Deleted finds.
type finder struct {
	db     *DB
	tables []*Table
	live   map[uint32]bool // pages of the tables' b-trees
	keys   map[string]bool // rowKeys of the rows still in the tables, and those found
	found  map[string]int  // rowKeys of those found, to their place in out
	out    []Record
}

// add keeps a record unless it's a row still in its table or found
// before. A row found again with its rowid gets it.
func (f *finder) add(t *Table, rowid int64, v []any, where string) {
	row := t.withRowid(rowid, v)
	keys := rowKeys(t, rowid, row)
	if rowid != 0 && f.keys[fmt.Sprintf("%s\x00%d", t.Name, rowid)] && len(keys) == 1 {
		// The table still has this rowid: an older version of a row
		// that was changed.
		return
	}
	for _, k := range keys {
		if f.keys[k] {
			if i, ok := f.found[k]; ok && rowid != 0 && f.out[i].Rowid == 0 {
				f.out[i].Rowid = rowid
				f.out[i].Values = t.withRowid(rowid, f.out[i].Values)
			}
			return
		}
	}
	for _, k := range keys {
		f.keys[k] = true
		f.found[k] = len(f.out)
	}
	if rowid != 0 {
		f.keys[fmt.Sprintf("%s\x00%d", t.Name, rowid)] = true
	}
	f.out = append(f.out, Record{Table: t.Name, Rowid: rowid, Values: row, Where: where})
}

// carve looks for records anywhere in p[from:end].
func (f *finder) carve(p []byte, from, end int, where string) {
	for o := from; o < end; {
		types, hl, ok := header(p, o, end)
		if !ok {
			// A freeblock's four bytes over the start of a cell, left
			// behind in space SQLite has since given back to the page.
			if size, ok := staleFreeblock(p, o-4); ok {
				if r, ok := f.headless(p, o, min(end, o-4+size), where); ok {
					o = r
					continue
				}
			}
			o++
			continue
		}
		body := 0
		for _, ty := range types {
			body += serialSize(ty)
		}
		if o+hl+body > end {
			o++
			continue
		}
		t := f.match(types, true)
		if t == nil {
			o++
			continue
		}
		v := f.decode(p[o+hl:o+hl+body], types)
		if !plausible(v, types) {
			o++
			continue
		}
		f.add(t, cellRowid(p, from, o, hl+body), v, where)
		o += hl + body
	}
}

// staleFreeblock tells whether p[at:at+4] looks like a freeblock's
// header, a link to the next and a size, and returns the size.
func staleFreeblock(p []byte, at int) (int, bool) {
	if at < 0 || at+4 > len(p) {
		return 0, false
	}
	next, size := int(binary.BigEndian.Uint16(p[at:])), int(binary.BigEndian.Uint16(p[at+2:]))
	if size < 8 || at+size > len(p) || next != 0 && (next <= at || next >= len(p)) {
		return 0, false
	}
	return size, true
}

// cellRowid reads the rowid of a whole cell whose record starts at
// p[o], payload bytes long, if its lengths are still there before it.
func cellRowid(p []byte, from, o, payload int) int64 {
	for b := o - 2; b >= max(from, o-18); b-- {
		size, k := varint(p[b:o])
		if k == 0 || size != uint64(payload) {
			continue
		}
		rowid, k2 := varint(p[b+k : o])
		if k2 > 0 && b+k+k2 == o && rowid > 0 && rowid < 1<<53 {
			return int64(rowid)
		}
	}
	return 0
}

// header reads a record header at p[o:end]: its serial types and its
// length.
func header(p []byte, o, end int) ([]uint64, int, bool) {
	hl, n := varint(p[o:end])
	if n == 0 || hl < uint64(n)+1 || hl > 2000 || o+int(hl) > end {
		return nil, 0, false
	}
	var types []uint64
	for i := o + n; i < o+int(hl); {
		t, k := varint(p[i : o+int(hl)])
		if k == 0 || t == 10 || t == 11 {
			return nil, 0, false
		}
		types = append(types, t)
		i += k
	}
	return types, int(hl), true
}
