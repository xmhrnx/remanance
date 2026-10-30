package sqlite

import (
	"encoding/binary"
	"fmt"
)

// Page types of a table b-tree.
const (
	interiorTable = 0x05
	leafTable     = 0x0d
)

// walk calls fn with every row of the table b-tree rooted at page root,
// in rowid order, until fn returns false.
func (db *DB) walk(root uint32, fn func(rowid int64, payload []byte) bool) error {
	seen := map[uint32]bool{}
	_, err := db.walkPage(root, seen, 0, fn)
	return err
}

func (db *DB) walkPage(n uint32, seen map[uint32]bool, depth int, fn func(int64, []byte) bool) (bool, error) {
	if seen[n] || depth > 40 {
		return false, fmt.Errorf("sqlite: the b-tree loops back on page %d", n)
	}
	seen[n] = true
	p, err := db.page(n)
	if err != nil {
		return false, err
	}
	h := 0
	if n == 1 {
		h = 100 // the file header comes first on page 1
	}
	if h+8 > len(p) {
		return false, ErrFormat
	}
	kind := p[h]
	cells := int(binary.BigEndian.Uint16(p[h+3:]))
	ptrs := h + 8
	if kind == interiorTable {
		ptrs = h + 12
	}
	if ptrs+2*cells > len(p) {
		return false, fmt.Errorf("sqlite: page %d lists more cells than fit", n)
	}
	switch kind {
	case interiorTable:
		for i := range cells {
			c := int(binary.BigEndian.Uint16(p[ptrs+2*i:]))
			if c+4 > len(p) {
				continue
			}
			more, err := db.walkPage(binary.BigEndian.Uint32(p[c:]), seen, depth+1, fn)
			if err != nil || !more {
				return more, err
			}
		}
		return db.walkPage(binary.BigEndian.Uint32(p[h+8:]), seen, depth+1, fn)
	case leafTable:
		for i := range cells {
			c := int(binary.BigEndian.Uint16(p[ptrs+2*i:]))
			if c >= len(p) {
				continue
			}
			size, k := varint(p[c:])
			rowid, k2 := varint(p[c+k:])
			if k == 0 || k2 == 0 {
				continue
			}
			payload, err := db.payload(p, c+k+k2, int64(size))
			if err != nil {
				return false, err
			}
			if !fn(int64(rowid), payload) {
				return false, nil
			}
		}
		return true, nil
	}
	return false, fmt.Errorf("sqlite: page %d isn't part of a table (type %#x)", n, kind)
}

// payload gathers a cell's payload of size bytes starting at p[at],
// following its overflow pages when it doesn't fit on the page.
func (db *DB) payload(p []byte, at int, size int64) ([]byte, error) {
	u := int64(db.usable)
	x := u - 35
	local := size
	if size > x {
		m := (u-12)*32/255 - 23
		local = m + (size-m)%(u-4)
		if local > x {
			local = m
		}
	}
	if int64(at)+local > int64(len(p)) || size > 1<<30 {
		return nil, fmt.Errorf("sqlite: a cell runs past its page")
	}
	out := make([]byte, 0, size)
	out = append(out, p[at:at+int(local)]...)
	if local == size {
		return out, nil
	}
	if at+int(local)+4 > len(p) {
		return nil, fmt.Errorf("sqlite: a cell runs past its page")
	}
	next := binary.BigEndian.Uint32(p[at+int(local):])
	for i := 0; int64(len(out)) < size; i++ {
		if next == 0 || i > int(db.pages) {
			return nil, fmt.Errorf("sqlite: an overflow chain ends early")
		}
		op, err := db.page(next)
		if err != nil {
			return nil, err
		}
		k := min(size-int64(len(out)), u-4)
		out = append(out, op[4:4+k]...)
		next = binary.BigEndian.Uint32(op)
	}
	return out, nil
}
