package sqlite

import (
	"strings"
	"unicode/utf8"
)

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
