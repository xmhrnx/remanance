package model

import (
	"context"
	"errors"

	"github.com/xmhrnx/remanance/engine/space"
	"github.com/xmhrnx/remanance/engine/trash"
	"github.com/xmhrnx/remanance/ui/platform/androidstore"
)

// SpaceState is the disk space tool: the folder measured and the one
// shown.
type SpaceState struct {
	Root      *space.Node
	Current   *space.Node
	Selected  *space.Node
	Measuring bool
	Progress  space.Progress
	Err       error
	cancel    context.CancelFunc
}

// MeasureSpace measures the folder or drive at path.
func (m *Model) MeasureSpace(path string) {
	if m.Space.Measuring {
		return
	}
	if err := androidstore.EnsureAccess(); err != nil {
		m.Space.Err = err
		return
	}
	s := &m.Space
	s.Measuring, s.Err, s.Progress = true, nil, space.Progress{}
	s.Root, s.Current, s.Selected = nil, nil, nil
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	release := keepRunning(m.App, "Measuring disk space")
	go func() {
		defer release()
		var progress latest[space.Progress]
		root, err := space.Measure(ctx, path, func(p space.Progress) {
			progress.post(m.App, p, func(p space.Progress) { s.Progress = p })
		})
		m.App.Post(func() {
			s.Measuring, s.cancel = false, nil
			if errors.Is(err, context.Canceled) {
				return
			}
			s.Err = err
			s.Root, s.Current = root, root
		})
	}()
}

// CancelSpace stops measuring.
func (m *Model) CancelSpace() {
	if m.Space.cancel != nil {
		m.Space.cancel()
	}
}

// OpenSpace shows a folder's contents.
func (m *Model) OpenSpace(n *space.Node) {
	if n == nil {
		return
	}
	if !n.IsDir {
		m.Space.Selected = n
		n = n.Parent
	} else {
		m.Space.Selected = nil
	}
	m.Space.Current = n
}

// CloseSpace goes back to choosing a folder.
func (m *Model) CloseSpace() {
	m.CancelSpace()
	m.Space = SpaceState{}
}

// TrashSpace moves a file or folder to the trash, or deletes it where
// there's no trash, and takes it out of the map.
func (m *Model) TrashSpace(n *space.Node) {
	if n == nil || n.Parent == nil {
		return
	}
	go func() {
		err := trash.Remove(n.Path, true)
		m.App.Post(func() {
			if err != nil {
				m.Space.Err = err
				return
			}
			if m.Space.Selected == n {
				m.Space.Selected = nil
			}
			if m.Space.Current == n {
				m.Space.Current = n.Parent
			}
			n.Remove()
		})
	}()
}
