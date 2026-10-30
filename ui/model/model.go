// Package model holds what the app shows and runs the engine's jobs on
// background goroutines. Fields are changed only on the UI goroutine:
// jobs hand results back through App.Post.
package model

import (
	"context"

	"github.com/mehran9675/giode"
	"github.com/xmhrnx/remanance/engine/disk"
	"github.com/xmhrnx/remanance/engine/fsys"
	"github.com/xmhrnx/remanance/engine/recovery"
	"github.com/xmhrnx/remanance/engine/scan"
)

// Screen is the page on show.
type Screen int

const (
	ScreenDrives Screen = iota
	ScreenScan
	ScreenResults
)

// Model is the whole app's state.
type Model struct {
	App    *giode.App
	Screen Screen

	// Drives
	Drives        []disk.Info
	DrivesLoading bool
	DrivesErr     error
	// NeedsAdmin is set when a drive couldn't be opened for lack of
	// rights, so the app offers to restart with them.
	NeedsAdmin bool
	Opening    bool
	OpenErr    error

	// The scan
	Source   disk.Info
	Dev      disk.Device
	Mode     scan.Mode
	Progress scan.Progress
	Scanning bool
	ScanErr  error
	Result   *scan.Result
	cancel   context.CancelFunc

	// Browsing the results
	Roots    []*Node
	Folder   *Node
	Preview  *fsys.Entry
	Checked  map[*fsys.Entry]bool
	Filter   Filter
	rowCache []*fsys.Entry
	rowKey   rowKey

	// Recovery
	Recovering bool
	RecoverP   recovery.Progress
	Report     *recovery.Report
	RecoverErr error
	recCancel  context.CancelFunc

	// Changed is called after a job changes the model, for views that
	// cache what they show.
	Changed func()
}

// New returns an empty model for app.
func New(app *giode.App) *Model {
	return &Model{App: app, Checked: map[*fsys.Entry]bool{}, Mode: scan.Quick}
}

// changed tells the views the model moved on.
func (m *Model) changed() {
	m.rowKey = rowKey{}
	if m.Changed != nil {
		m.Changed()
	}
}

// Close releases the open device.
func (m *Model) Close() {
	m.CancelScan()
	m.CancelRecover()
	if m.Dev != nil {
		m.Dev.Close()
		m.Dev = nil
	}
}
