package model

import (
	"context"
	"errors"
	"time"

	"github.com/xmhrnx/remanance/engine/disk"
	"github.com/xmhrnx/remanance/engine/fsys"
	"github.com/xmhrnx/remanance/engine/scan"
	"github.com/xmhrnx/remanance/locales"
)

// ScanFolder scans the drive a folder is on for what was deleted from
// that folder.
func (m *Model) ScanFolder(path string, mode scan.Mode) {
	info, within, ok := disk.VolumeFor(m.Drives, path)
	if !ok {
		m.OpenErr = errors.New(locales.T("scan.folder_not_on_a_drive"))
		return
	}
	m.Open(info, func() {
		m.Within = within
		m.StartScan(mode)
	})
}

// StartScan scans the open device in mode, within the folder set.
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
	release := keepRunning(m.App, locales.T("scan.scanning_for_deleted_files"))
	go func() {
		defer release()
		var progress latest[scan.Progress]
		res, err := scan.Run(ctx, dev, scan.Options{Mode: mode, Folder: m.Within}, func(p scan.Progress) {
			progress.post(m.App, p, func(p scan.Progress) {
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
