package model

import (
	"context"
	"errors"
	"slices"

	"github.com/xmhrnx/remanance/engine/dupes"
	"github.com/xmhrnx/remanance/locales"
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
	release := keepRunning(m.App, locales.T("dupes.looking_for_duplicates"))
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
