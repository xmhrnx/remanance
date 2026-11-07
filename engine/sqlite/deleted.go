package sqlite

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// Record is a row found in the database's free space: deleted, and not
// yet written over.
type Record struct {
	Table  string
	Rowid  int64 // 0 when the start of its cell was written over
	Values []any // in the table's column order; nil where unknown
	Where  string
}

// Where a Record was found.
const (
	InFreePage   = "a free page"
	InFreeSpace  = "free space in a page"
	InUnusedPage = "a page no longer used"
	InWAL        = "the write-ahead log"
)

// Deleted finds rows deleted from the database's tables. SQLite leaves a
// deleted row's bytes where they were until something else needs the
// space: in the free space of the page that held it, or in a whole page
// put on the free list, or in an older copy of the page in the
// write-ahead log. Rows found without their cell's start are matched to
// a table by their columns' types. Rows still in the tables, and older
// versions of them, are left out.
func (db *DB) Deleted() ([]Record, error) {
	tables, err := db.Tables()
	if err != nil {
		return nil, err
	}
	f := &finder{db: db, live: map[uint32]bool{}, keys: map[string]bool{}, found: map[string]int{}}
	for _, t := range tables {
		if len(t.Columns) == 0 || strings.HasPrefix(strings.ToLower(t.Name), "sqlite_") {
			continue // SQLite's own, such as sqlite_sequence
		}
		f.tables = append(f.tables, t)
		err := db.walkInto(t.Root, f.live, func(rowid int64, payload []byte) bool {
			for _, k := range rowKeys(t, rowid, t.withRowid(rowid, db.record(payload))) {
				f.keys[k] = true
			}
			f.keys[fmt.Sprintf("%s\x00%d", t.Name, rowid)] = true
			return true
		})
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", t.Name, err)
		}
	}
	db.walkInto(1, f.live, func(int64, []byte) bool { return true })
	free := db.freelist()

	for n := uint32(1); n <= db.pages; n++ {
		p, err := db.page(n)
		if err != nil {
			continue
		}
		h := 0
		if n == 1 {
			h = 100
		}
		switch {
		case f.live[n]:
			if p[h] == leafTable {
				f.freeSpace(p, h, InFreeSpace)
			}
		case free[n] == trunkPage:
			from := 8 + 4*int(binary.BigEndian.Uint32(p[4:]))
			f.carve(p, min(from, len(p)), len(p), InFreePage)
		case free[n] != 0:
			// A page that held an index or the inside of a b-tree holds
			// no rows; one that held overflowing data may hold their end.
			if !f.leaf(p, 0, InFreePage) && p[0] != interiorTable && p[0] != 0x02 && p[0] != 0x0a {
				f.carve(p, 0, len(p), InFreePage)
			}
		case p[h] == leafTable:
			f.leaf(p, h, InUnusedPage)
		}
	}
	for _, fr := range db.frames {
		h := 0
		if fr.page == 1 {
			h = 100
		}
		if !f.leaf(fr.data, h, InWAL) {
			f.carve(fr.data, h, len(fr.data), InWAL)
		}
	}
	return f.out, nil
}
