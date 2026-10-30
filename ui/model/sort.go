package model

import (
	"cmp"
	"slices"
	"strings"

	"github.com/xmhrnx/remanance/engine/fsys"
)

// Column is a column of the table the files can be sorted by.
type Column int

const (
	ColName Column = iota
	ColSize
	ColModified
	ColState
	ColHealth
)

// Order is how the table is sorted. Folders stay above files either
// way, and files that tie are ordered by name.
type Order struct {
	Column Column
	Desc   bool
}

// SortChoice is one way to sort, as offered on a phone, where the table
// has no header to click.
type SortChoice struct {
	Label string
	Order Order
}

// SortChoices are the ways to sort offered on a phone.
var SortChoices = []SortChoice{
	{"Name, A to Z", Order{ColName, false}},
	{"Name, Z to A", Order{ColName, true}},
	{"Largest first", Order{ColSize, true}},
	{"Smallest first", Order{ColSize, false}},
	{"Newest first", Order{ColModified, true}},
	{"Oldest first", Order{ColModified, false}},
	{"Deleted first", Order{ColState, false}},
	{"Existing first", Order{ColState, true}},
	{"Best recovery first", Order{ColHealth, true}},
	{"Worst recovery first", Order{ColHealth, false}},
}

// SortBy sorts by a column, as when its header is clicked: the column
// already sorted by flips direction, and another starts the way people
// usually want it, largest, newest and most recoverable first.
func (m *Model) SortBy(c Column) {
	if m.Order.Column == c {
		m.Order.Desc = !m.Order.Desc
		return
	}
	m.Order = Order{Column: c, Desc: c == ColSize || c == ColModified || c == ColHealth}
}

// stateRank puts deleted files before the ones found other ways, and
// existing files last.
var stateRank = map[fsys.State]int{fsys.Deleted: 0, fsys.Orphan: 1, fsys.Carved: 2, fsys.Existing: 3}

func (o Order) compare(a, b *fsys.Entry) int {
	if a.IsDir != b.IsDir {
		if a.IsDir {
			return -1
		}
		return 1
	}
	var c int
	switch o.Column {
	case ColSize:
		c = cmp.Compare(a.Size, b.Size)
	case ColModified:
		c = a.Modified.Compare(b.Modified)
	case ColState:
		c = cmp.Compare(stateRank[a.State], stateRank[b.State])
	case ColHealth:
		c = cmp.Compare(a.Health, b.Health)
	}
	if c == 0 {
		c = cmp.Or(cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)), cmp.Compare(a.Name, b.Name))
		if o.Column != ColName {
			// Ties read A to Z whichever way the column runs.
			return c
		}
	}
	if o.Desc {
		return -c
	}
	return c
}

// sortRows sorts the table's rows in place.
func (o Order) sortRows(rows []*fsys.Entry) {
	slices.SortStableFunc(rows, o.compare)
}
