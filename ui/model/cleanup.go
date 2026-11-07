package model

import (
	"context"
	"errors"

	"github.com/xmhrnx/remanance/engine/cleanup"
	"github.com/xmhrnx/remanance/locales"
	"github.com/xmhrnx/remanance/ui/platform/androidstore"
)

// CleanState is the clean up tool: how full the drive is, what can go
// and what's checked to go.
type CleanState struct {
	Space    cleanup.Space
	Looked   bool // a search has run, or is running
	Looking  bool
	Progress cleanup.Progress
	Groups   []cleanup.Group
	// Checked has the paths checked to clean.
	Checked  map[string]bool
	Cleaning bool
	Cleaned  [2]int // items done, of
	Err      error
	// Done says what the last clean up did, or Failed why it couldn't.
	Done   string
	Failed error
	cancel context.CancelFunc
}

// FindCleanup looks for files that can go, checking those that are safe
// to clean without looking.
func (m *Model) FindCleanup() {
	c := &m.Clean
	if c.Looking || c.Cleaning {
		return
	}
	c.Looked = true
	c.Space, _ = cleanup.Free(cleanup.Drive())
	if err := androidstore.EnsureAccess(); err != nil {
		c.Err = err
		return
	}
	c.Looking, c.Err, c.Progress = true, nil, cleanup.Progress{}
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	release := keepRunning(m.App, locales.T("cleanup.looking"))
	go func() {
		defer release()
		var progress latest[cleanup.Progress]
		groups, err := cleanup.Find(ctx, func(p cleanup.Progress) {
			progress.post(m.App, p, func(p cleanup.Progress) { c.Progress = p })
		})
		m.App.Post(func() {
			c.Looking, c.cancel = false, nil
			if errors.Is(err, context.Canceled) {
				return
			}
			c.Err, c.Groups = err, groups
			c.Checked = map[string]bool{}
			for _, g := range groups {
				if g.Kind.Safe() {
					for _, it := range g.Items {
						c.Checked[it.Path] = true
					}
				}
			}
		})
	}()
}

// CancelCleanup stops looking.
func (m *Model) CancelCleanup() {
	if m.Clean.cancel != nil {
		m.Clean.cancel()
	}
}

// ToggleCleanItem checks or unchecks one item.
func (m *Model) ToggleCleanItem(path string) {
	if m.Clean.Checked[path] {
		delete(m.Clean.Checked, path)
	} else {
		m.Clean.Checked[path] = true
	}
}

// ToggleCleanGroup checks all of a group, or unchecks it when it's all
// checked.
func (m *Model) ToggleCleanGroup(g cleanup.Group) {
	all := m.CleanGroupChecked(g) == len(g.Items)
	for _, it := range g.Items {
		if all {
			delete(m.Clean.Checked, it.Path)
		} else {
			m.Clean.Checked[it.Path] = true
		}
	}
}

// CleanUp cleans the checked items, then looks again.
func (m *Model) CleanUp() {
	c := &m.Clean
	items, _ := m.CleanChecked()
	if c.Cleaning || len(items) == 0 {
		return
	}
	c.Cleaning, c.Failed, c.Done, c.Cleaned = true, nil, "", [2]int{0, len(items)}
	release := keepRunning(m.App, locales.T("cleanup.cleaning"))
	go func() {
		defer release()
		var progress latest[[2]int]
		r, err := cleanup.Clean(context.Background(), items, func(done, of int) {
			progress.post(m.App, [2]int{done, of}, func(p [2]int) { c.Cleaned = p })
		})
		m.App.Post(func() {
			c.Cleaning, c.Failed = false, err
			if err == nil {
				c.Done = cleanupDone(r)
			}
			m.FindCleanup()
		})
	}()
}
