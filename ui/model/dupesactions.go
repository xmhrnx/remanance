package model

import (
	"fmt"

	"github.com/xmhrnx/remanance/engine/dupes"
	"github.com/xmhrnx/remanance/engine/trash"
	"github.com/xmhrnx/remanance/locales"
)

// ToggleDupe checks or unchecks a file for removal. Checking every file
// of a group would lose it, so the last one can't be checked.
func (m *Model) ToggleDupe(g dupes.Group, path string) {
	d := &m.Dupes
	if d.Remove[path] {
		delete(d.Remove, path)
		return
	}
	left := 0
	for _, f := range g.Files {
		if !d.Remove[f.Path] && f.Path != path {
			left++
		}
	}
	if left > 0 {
		d.Remove[path] = true
	}
}

// DupesChecked counts the files checked and their size.
func (m *Model) DupesChecked() (int, int64) {
	n, size := 0, int64(0)
	for _, g := range m.Dupes.Groups {
		for _, f := range g.Files {
			if m.Dupes.Remove[f.Path] {
				n++
				size += f.Size
			}
		}
	}
	return n, size
}

// RemoveDupes moves the checked files to the trash, or deletes them
// where there's none, and takes them out of the groups.
func (m *Model) RemoveDupes() {
	d := &m.Dupes
	if d.Removing {
		return
	}
	var paths []string
	for p, on := range d.Remove {
		if on {
			paths = append(paths, p)
		}
	}
	if len(paths) == 0 {
		return
	}
	d.Removing, d.Err, d.Done = true, nil, ""
	go func() {
		removed := map[string]bool{}
		var failed error
		for _, p := range paths {
			if err := trash.Remove(p, true); err != nil {
				failed = err
				continue
			}
			removed[p] = true
		}
		m.App.Post(func() {
			d.Removing = false
			var freed int64
			var groups []dupes.Group
			for _, g := range d.Groups {
				var left []dupes.File
				for _, f := range g.Files {
					if removed[f.Path] {
						freed += f.Size
						delete(d.Remove, f.Path)
					} else {
						left = append(left, f)
					}
				}
				g.Files = left
				if len(left) > 1 {
					groups = append(groups, g)
				}
			}
			d.Groups = groups
			where := locales.T("dupes.moved_to_the_trash")
			if !trash.Supported {
				where = locales.T("dupes.deleted")
			}
			d.Done = locales.N("dupes.removed", len(removed), "where", where, "size", bytes(freed))
			if failed != nil {
				d.Err = fmt.Errorf("%s: %w", locales.T("dupes.some_not_removed"), failed)
			}
		})
	}()
}
