package sqlite

import (
	"encoding/binary"
	"fmt"
	"strings"
	"unicode/utf8"
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

// trunkPage marks a free list trunk page in freelist's map.
const trunkPage = -1

// freelist maps the pages on the free list: trunkPage for the trunk
// pages, which list the others, and 1 for those.
func (db *DB) freelist() map[uint32]int {
	out := map[uint32]int{}
	p1, err := db.page(1)
	if err != nil {
		return out
	}
	trunk := binary.BigEndian.Uint32(p1[32:])
	for i := 0; trunk != 0 && trunk <= db.pages && i < int(db.pages); i++ {
		if _, seen := out[trunk]; seen {
			break
		}
		out[trunk] = trunkPage
		p, err := db.page(trunk)
		if err != nil {
			break
		}
		n := int(binary.BigEndian.Uint32(p[4:]))
		for j := 0; j < n && 8+4*j+4 <= len(p); j++ {
			if leaf := binary.BigEndian.Uint32(p[8+4*j:]); leaf != 0 && leaf <= db.pages {
				out[leaf] = 1
			}
		}
		trunk = binary.BigEndian.Uint32(p)
	}
	return out
}

// walkInto walks a table b-tree like walk, adding its pages to seen.
func (db *DB) walkInto(root uint32, seen map[uint32]bool, fn func(int64, []byte) bool) error {
	_, err := db.walkPage(root, seen, 0, fn)
	return err
}

// withRowid puts the rowid in the column that aliases it, which the
// file stores as NULL.
func (t *Table) withRowid(rowid int64, v []any) []any {
	row := make([]any, max(len(t.Columns), len(v)))
	copy(row, v)
	if rowid != 0 && t.rowidCol >= 0 && t.rowidCol < len(row) && row[t.rowidCol] == nil {
		row[t.rowidCol] = rowid
	}
	return row
}

// rowKeys are what tell a row found apart from the rows still in the
// table: all its values, and, for an older version of a row since
// changed, its text with its rowid, or its text alone when there's
// enough of it to be unlikely twice.
func rowKeys(t *Table, rowid int64, row []any) []string {
	var all, text strings.Builder
	n := 0
	fmt.Fprintf(&all, "%s\x00", t.Name)
	for i, v := range row {
		if i == t.rowidCol {
			continue
		}
		fmt.Fprintf(&all, "%T:%v\x00", v, v)
		switch v := v.(type) {
		case string:
			text.WriteString(v)
			n += len(v)
		case []byte:
			text.Write(v)
			n += len(v)
		}
		text.WriteByte(0)
	}
	keys := []string{all.String()}
	if n > 0 && rowid != 0 {
		keys = append(keys, fmt.Sprintf("%s\x00%d\x00%s", t.Name, rowid, text.String()))
	}
	if n >= 16 {
		keys = append(keys, t.Name+"\x00text\x00"+text.String())
	}
	return keys
}

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

// match finds the table whose columns these serial types fit best: all
// the columns, or all but the last few, which an older row may lack.
func (f *finder) match(types []uint64, strict bool) *Table {
	var best *Table
	for _, t := range f.tables {
		if !t.fits(types, strict) {
			continue
		}
		if best == nil || len(t.Columns) == len(types) && len(best.Columns) != len(types) {
			best = t
		}
	}
	return best
}

// fits tells whether a record with these serial types could be a row of
// t. strict, for records found without their cell, asks for more.
func (t *Table) fits(types []uint64, strict bool) bool {
	n := len(t.Columns)
	if len(types) > n || len(types) == 0 {
		return false
	}
	if strict && (len(types) < n-3 || len(types) < 2) {
		return false
	}
	if strict && !t.typed() {
		return false // any bytes that parse would fit
	}
	nonNull := 0
	for i, ty := range types {
		if i == t.rowidCol && ty != 0 {
			return false
		}
		if !compatible(affinity(t.Types[i]), ty) {
			return false
		}
		if ty != 0 {
			nonNull++
		}
	}
	return !strict || nonNull > 0
}

// typed tells whether any of t's columns has a type that limits what
// it holds.
func (t *Table) typed() bool {
	for _, d := range t.Types {
		if affinity(d) != 'b' {
			return true
		}
	}
	return false
}

// affinity is SQLite's type affinity for a declared type: 'i'nteger,
// 't'ext, 'b'lob (or none), 'r'eal or 'n'umeric.
func affinity(decl string) byte {
	switch {
	case strings.Contains(decl, "INT"):
		return 'i'
	case strings.Contains(decl, "CHAR"), strings.Contains(decl, "CLOB"), strings.Contains(decl, "TEXT"):
		return 't'
	case decl == "", strings.Contains(decl, "BLOB"):
		return 'b'
	case strings.Contains(decl, "REAL"), strings.Contains(decl, "FLOA"), strings.Contains(decl, "DOUB"):
		return 'r'
	}
	return 'n'
}

// compatible tells whether a column of this affinity can hold a value
// of serial type t.
func compatible(aff byte, t uint64) bool {
	switch {
	case t == 0:
		return true
	case aff == 'b' || aff == 'n':
		return true
	case aff == 'i' || aff == 'r':
		return t <= 9
	case aff == 't':
		return t >= 13 && t%2 == 1
	}
	return false
}

func (f *finder) values(payload []byte, hl int, types []uint64) []any {
	return f.decode(payload[hl:], types)
}

// decode reads a record's body; what doesn't fit reads as nil.
func (f *finder) decode(body []byte, types []uint64) []any {
	out := make([]any, len(types))
	for i, t := range types {
		size := serialSize(t)
		if size > len(body) {
			break
		}
		out[i] = f.db.value(t, body[:size])
		body = body[size:]
	}
	return out
}

// plausible weeds out bytes that only happen to parse as a record: its
// text has to be text, and it has to say something.
func plausible(v []any, types []uint64) bool {
	some := false
	for i, x := range v {
		switch x := x.(type) {
		case string:
			if !utf8.ValidString(x) {
				return false
			}
			for _, r := range x {
				if r < 0x20 && r != '\n' && r != '\r' && r != '\t' || r == utf8.RuneError {
					return false
				}
			}
			some = some || x != ""
		case []byte:
			some = some || len(x) > 0
		case nil:
		default:
			some = some || types[i] != 8
		}
	}
	return some
}
