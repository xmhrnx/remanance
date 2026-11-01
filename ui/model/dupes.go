package model

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/xmhrnx/remanance/engine/dupes"
	"github.com/xmhrnx/remanance/engine/trash"
	"github.com/xmhrnx/remanance/ui/platform/androidstore"
)

// DupesState is the duplicate finder: where to look, how, and what it
// found.
type DupesState struct {
	Roots     []string
	Similar   bool
	Threshold int
	Running   bool
	Progress  dupes.Progress
	Groups    []dupes.Group
	// Remove has the paths checked for removal.
	Remove   map[string]bool
	Removing bool
	Err      error
	// Done says what the last removal did.
	Done   string
	cancel context.CancelFunc
}

// AddDupesRoot adds a folder to look through.
func (m *Model) AddDupesRoot(dir string) {
	if dir != "" && !slices.Contains(m.Dupes.Roots, dir) {
		m.Dupes.Roots = append(m.Dupes.Roots, dir)
	}
}

// RemoveDupesRoot stops looking through a folder.
func (m *Model) RemoveDupesRoot(dir string) {
	m.Dupes.Roots = slices.DeleteFunc(m.Dupes.Roots, func(s string) bool { return s == dir })
}

// FindDupes looks through the folders for files that are alike.
func (m *Model) FindDupes() {
	d := &m.Dupes
	if d.Running || len(d.Roots) == 0 {
		return
	}
	if err := androidstore.EnsureAccess(); err != nil {
		d.Err = err
		return
	}
	d.Running, d.Err, d.Done, d.Groups = true, nil, "", nil
	d.Progress = dupes.Progress{}
	d.Remove = map[string]bool{}
	ctx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel
	roots, opts := slices.Clone(d.Roots), dupes.Options{Similar: d.Similar, Threshold: d.Threshold}
	release := keepRunning(m.App, "Looking for duplicates")
	go func() {
		defer release()
		var progress latest[dupes.Progress]
		groups, err := dupes.Find(ctx, roots, opts, func(p dupes.Progress) {
			progress.post(m.App, p, func(p dupes.Progress) { d.Progress = p })
		})
		m.App.Post(func() {
			d.Running, d.cancel = false, nil
			if errors.Is(err, context.Canceled) {
				return
			}
			d.Err, d.Groups = err, groups
			// Everything but the copy to keep starts checked.
			for _, g := range groups {
				for _, f := range g.Files {
					if !f.Keep {
						d.Remove[f.Path] = true
					}
				}
			}
		})
	}()
}

// CancelDupes stops looking.
func (m *Model) CancelDupes() {
	if m.Dupes.cancel != nil {
		m.Dupes.cancel()
	}
}

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
			where := "moved to the trash"
			if !trash.Supported {
				where = "deleted"
			}
			d.Done = fmt.Sprintf("%d %s %s, freeing %s", len(removed), plural(len(removed), "file", "files"), where, bytes(freed))
			if failed != nil {
				d.Err = fmt.Errorf("some files couldn't be removed: %w", failed)
			}
		})
	}()
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
