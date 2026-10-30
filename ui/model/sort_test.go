package model

import (
	"slices"
	"testing"
	"time"

	"github.com/xmhrnx/remanance/engine/fsys"
)

func TestOrder(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, 1, d, 0, 0, 0, 0, time.UTC) }
	rows := []*fsys.Entry{
		{Name: "b.jpg", Size: 300, Modified: day(3), State: fsys.Existing, Health: 100},
		{Name: "Photos", IsDir: true},
		{Name: "a.jpg", Size: 100, Modified: day(1), State: fsys.Deleted, Health: 40},
		{Name: "c.jpg", Size: 300, Modified: day(2), State: fsys.Carved, Health: 90},
	}
	names := func(o Order) []string {
		r := slices.Clone(rows)
		o.sortRows(r)
		var out []string
		for _, e := range r {
			out = append(out, e.Name)
		}
		return out
	}
	for _, tc := range []struct {
		o    Order
		want []string
	}{
		{Order{ColName, false}, []string{"Photos", "a.jpg", "b.jpg", "c.jpg"}},
		{Order{ColName, true}, []string{"Photos", "c.jpg", "b.jpg", "a.jpg"}},
		// Equal sizes stay A to Z, whichever way the sizes run.
		{Order{ColSize, true}, []string{"Photos", "b.jpg", "c.jpg", "a.jpg"}},
		{Order{ColSize, false}, []string{"Photos", "a.jpg", "b.jpg", "c.jpg"}},
		{Order{ColModified, true}, []string{"Photos", "b.jpg", "c.jpg", "a.jpg"}},
		{Order{ColState, false}, []string{"Photos", "a.jpg", "c.jpg", "b.jpg"}},
		{Order{ColHealth, true}, []string{"Photos", "b.jpg", "c.jpg", "a.jpg"}},
	} {
		if got := names(tc.o); !slices.Equal(got, tc.want) {
			t.Errorf("%+v: got %v, want %v", tc.o, got, tc.want)
		}
	}
}

func TestSortBy(t *testing.T) {
	var m Model
	m.SortBy(ColSize)
	if m.Order != (Order{ColSize, true}) {
		t.Fatalf("size starts largest first, got %+v", m.Order)
	}
	m.SortBy(ColSize)
	if m.Order != (Order{ColSize, false}) {
		t.Fatalf("clicking again flips, got %+v", m.Order)
	}
	m.SortBy(ColName)
	if m.Order != (Order{ColName, false}) {
		t.Fatalf("name starts A to Z, got %+v", m.Order)
	}
}
