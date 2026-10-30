package model

import (
	"context"
	"time"

	"github.com/mehran9675/remanence/engine/fsys"
	"github.com/mehran9675/remanence/engine/scan"
)

// StartScan scans the open device in mode.
func (m *Model) StartScan(mode scan.Mode) {
	if m.Dev == nil || m.Scanning {
		return
	}
	m.Mode = mode
	m.Scanning, m.ScanErr, m.Result = true, nil, nil
	m.Progress = scan.Progress{Phase: scan.PhaseTable}
	m.Roots, m.Folder, m.Preview = nil, nil, nil
	m.Checked = map[*fsys.Entry]bool{}
	m.Report = nil
	m.Screen = ScreenScan
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	dev := m.Dev
	started := time.Now()
	go func() {
		res, err := scan.Run(ctx, dev, scan.Options{Mode: mode}, func(p scan.Progress) {
			m.App.Post(func() {
				if m.Scanning {
					m.Progress = p
				}
			})
		})
		m.App.Post(func() {
			m.Scanning, m.cancel = false, nil
			m.ScanErr = err
			if res != nil {
				if res.Duration == 0 {
					res.Duration = time.Since(started)
				}
				m.Result = res
				m.Roots = roots(res)
				if len(m.Roots) > 0 {
					m.Folder = m.Roots[0]
				}
				m.Screen = ScreenResults
			}
			m.changed()
		})
	}()
}

// CancelScan stops a running scan; what was found so far is kept.
func (m *Model) CancelScan() {
	if m.cancel != nil {
		m.cancel()
	}
}

// NewScan goes back to the drive list.
func (m *Model) NewScan() {
	m.CancelScan()
	m.Screen = ScreenDrives
	m.LoadDrives()
}
