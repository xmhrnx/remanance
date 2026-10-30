package model

import "github.com/xmhrnx/remanance/engine/fsys"

// Toggle checks or unchecks a file or folder for recovery; a folder
// stands for everything recoverable in it.
func (m *Model) Toggle(e *fsys.Entry) {
	if m.Checked[e] {
		delete(m.Checked, e)
	} else {
		m.Checked[e] = true
	}
}

// IsChecked reports whether e, or a folder above it, is checked.
func (m *Model) IsChecked(e *fsys.Entry) bool {
	for c := e; c != nil; c = c.Parent {
		if m.Checked[c] {
			return true
		}
	}
	return false
}

// CheckAll checks every row of the table, or clears them all when they
// are already checked.
func (m *Model) CheckAll() {
	rows := m.Rows()
	all := len(rows) > 0
	for _, e := range rows {
		if !m.IsChecked(e) {
			all = false
			break
		}
	}
	for _, e := range rows {
		if all {
			delete(m.Checked, e)
		} else {
			m.Checked[e] = true
		}
	}
}

// ClearChecked unchecks everything.
func (m *Model) ClearChecked() { m.Checked = map[*fsys.Entry]bool{} }

// Selection counts the files that would be recovered and their size.
// Inside a checked folder only what the current filter shows counts,
// so "deleted only" recovers only deleted files.
func (m *Model) Selection() (files int, size int64) {
	m.eachChecked(func(e *fsys.Entry) {
		files++
		size += e.Size
	})
	return files, size
}

func (m *Model) eachChecked(fn func(*fsys.Entry)) {
	seen := map[*fsys.Entry]bool{}
	f := m.Filter
	f.Query = ""
	for top := range m.Checked {
		top.Walk(func(e *fsys.Entry) bool {
			if e.IsDir {
				return e == top || f.keep(e) || f.Show == ShowAll
			}
			if !seen[e] && e.Recoverable() && (e == top || f.keep(e)) {
				seen[e] = true
				fn(e)
			}
			return true
		})
	}
}
