package model

import (
	"context"
	"errors"

	"github.com/xmhrnx/remanance/engine/iosbackup"
	"github.com/xmhrnx/remanance/engine/iphone"
)

// LoadPhones lists the iPhones connected by USB, on desktops, in the
// background.
func (m *Model) LoadPhones() {
	go func() {
		phones, err := iphone.Connected()
		m.App.Post(func() {
			m.Phones = phones
			m.PhoneService = !errors.Is(err, iphone.ErrNoService)
		})
	}()
}

// BackUpPhone backs up a connected iPhone into Remanence's own folder of
// backups, then lists it with the others and runs then with its ID.
func (m *Model) BackUpPhone(p iphone.Phone, then func(id string)) {
	if m.BackingUp != nil {
		return
	}
	m.BackingUp = &iphone.Progress{Step: iphone.StepStart}
	m.BackupErr = nil
	ctx, cancel := context.WithCancel(context.Background())
	m.backupCancel = cancel
	go func() {
		dir, err := iphone.Backup(ctx, p, iosbackup.OwnPlace(), func(pr iphone.Progress) {
			m.App.Post(func() {
				if m.BackingUp != nil {
					m.BackingUp = &pr
				}
			})
		})
		m.App.Post(func() {
			m.BackingUp, m.backupCancel = nil, nil
			if err != nil {
				if !errors.Is(err, context.Canceled) {
					m.BackupErr = err
				}
				return
			}
			m.LoadDrives()
			then(iosbackup.Info(iosbackup.Backup{Dir: dir}).ID)
		})
	}()
}

// CancelPhoneBackup stops a backup; the phone sends the rest next time.
func (m *Model) CancelPhoneBackup() {
	if m.backupCancel != nil {
		m.backupCancel()
	}
}
