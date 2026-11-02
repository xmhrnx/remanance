package model

import (
	"context"
	"errors"

	"github.com/xmhrnx/remanance/engine/disk"
	"github.com/xmhrnx/remanance/engine/fsys"
	"github.com/xmhrnx/remanance/engine/recovery"
	"github.com/xmhrnx/remanance/locales"
)

// ErrSameDrive refuses a destination on the drive being recovered:
// writing there could overwrite the very files being saved.
var ErrSameDrive = errors.New(locales.T("recover.that_folder_is_on_the_drive"))

// CheckDestination reports why dest can't take the recovered files.
func (m *Model) CheckDestination(dest string) error {
	if dest == "" {
		return errors.New(locales.T("recover.choose_a_folder_to_save_the"))
	}
	if recovery.SameDevice(m.Source, dest) {
		return ErrSameDrive
	}
	return nil
}

// StartRecover copies the checked files into sink.
func (m *Model) StartRecover(sink recovery.Sink, keepFolders bool) {
	if m.Recovering || m.Dev == nil {
		return
	}
	var items []recovery.Item
	m.eachChecked(func(e *fsys.Entry) {
		items = append(items, recovery.Item{Entry: e, Prefix: m.prefixOf(e)})
	})
	if len(items) == 0 {
		return
	}
	m.Recovering, m.RecoverErr, m.Report = true, nil, nil
	m.RecoverP = recovery.Progress{TotalFiles: len(items)}
	ctx, cancel := context.WithCancel(context.Background())
	m.recCancel = cancel
	dev := m.Dev
	release := keepRunning(m.App, locales.T("recover.recovering_files"))
	go func() {
		defer release()
		var progress latest[recovery.Progress]
		rep, err := recovery.Recover(ctx, dev, items, sink, recovery.Options{KeepFolders: keepFolders, Report: true}, func(p recovery.Progress) {
			progress.post(m.App, p, func(p recovery.Progress) { m.RecoverP = p })
		})
		m.App.Post(func() {
			m.Recovering, m.recCancel = false, nil
			m.Report, m.RecoverErr = rep, err
			if errors.Is(err, context.Canceled) {
				m.RecoverErr = errors.New(locales.N("recover.stopped_after", rep.Recovered))
			}
			if disk.IsGone(err) {
				m.RecoverErr = errors.New(locales.T("recover.the_drive_disconnected_plug_it_back"))
			}
			m.changed()
		})
	}()
}

// CancelRecover stops a running recovery after the file in progress.
func (m *Model) CancelRecover() {
	if m.recCancel != nil {
		m.recCancel()
	}
}

// prefixOf is the folder named after the volume a file came from.
func (m *Model) prefixOf(e *fsys.Entry) string {
	top := e
	for top.Parent != nil {
		top = top.Parent
	}
	for _, r := range m.Roots {
		if r.Entry == top {
			return r.prefix()
		}
	}
	return ""
}
