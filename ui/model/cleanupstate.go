package model

import (
	"github.com/xmhrnx/remanance/engine/cleanup"
	"github.com/xmhrnx/remanance/locales"
)

// CleanGroupChecked counts a group's checked items.
func (m *Model) CleanGroupChecked(g cleanup.Group) int {
	n := 0
	for _, it := range g.Items {
		if m.Clean.Checked[it.Path] {
			n++
		}
	}
	return n
}

// CleanChecked is the checked items, and their size.
func (m *Model) CleanChecked() ([]cleanup.Item, int64) {
	var items []cleanup.Item
	var size int64
	for _, g := range m.Clean.Groups {
		for _, it := range g.Items {
			if m.Clean.Checked[it.Path] {
				items = append(items, it)
				size += it.Size
			}
		}
	}
	return items, size
}

// cleanupDone says what a clean up did.
func cleanupDone(r cleanup.Result) string {
	msg := locales.T("cleanup.freed", "size", bytes(r.Freed))
	if r.Trashed > 0 {
		msg += " " + locales.T("cleanup.trashed", "size", bytes(r.Trashed))
	}
	if r.Left > 0 {
		msg += " " + locales.N("cleanup.left", r.Left)
	}
	return msg
}
