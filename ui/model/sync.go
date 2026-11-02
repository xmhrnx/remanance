package model

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gioui.org/app"

	"github.com/xmhrnx/remanance/engine/devsync"
	"github.com/xmhrnx/remanance/ui/platform/androidstore"
)

// SyncState is syncing with the user's other devices.
type SyncState struct {
	Node    *devsync.Node
	Status  devsync.Status
	Nearby  []devsync.Nearby
	Err     error
	release func()
	// Background keeps syncing while the app is out of sight.
	Background bool
}

// syncDir is where the device's key, settings and indexes are kept.
func syncDir() (string, error) {
	dir, err := app.DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "DataDoc", "sync"), nil
}

// deviceName is what other devices call this one at first.
func deviceName() string {
	if h, err := os.Hostname(); err == nil && h != "" && h != "localhost" {
		return strings.TrimSuffix(h, ".local")
	}
	switch runtime.GOOS {
	case "android":
		return "Android phone"
	case "ios":
		return "iPhone"
	}
	return "Computer"
}

// SyncConfigured tells whether this device syncs any folders, so syncing
// starts with the app.
func SyncConfigured() bool {
	dir, err := syncDir()
	if err != nil {
		return false
	}
	b, err := os.ReadFile(filepath.Join(dir, "config.json"))
	return err == nil && strings.Contains(string(b), `"path"`)
}

// StartSync starts syncing, once.
func (m *Model) StartSync() {
	s := &m.Sync
	if s.Node != nil {
		return
	}
	dir, err := syncDir()
	if err != nil {
		s.Err = err
		return
	}
	n, err := devsync.Open(dir, deviceName())
	if err != nil {
		s.Err = err
		return
	}
	n.OnChange(func() { m.App.Post(m.refreshSync) })
	if err := n.Start(context.Background(), devsync.Options{Discovery: true}); err != nil {
		s.Err = err
		return
	}
	s.Node = n
	m.refreshSync()
}

// StopSync stops syncing and disconnects.
func (m *Model) StopSync() {
	s := &m.Sync
	if s.Node != nil {
		s.Node.Close()
		s.Node = nil
	}
	if s.release != nil {
		s.release()
		s.release = nil
	}
}

func (m *Model) refreshSync() {
	s := &m.Sync
	if s.Node == nil {
		return
	}
	s.Status = s.Node.Status()
	s.Nearby = s.Node.Nearby()
}

// RefreshSync reads the status again, for the views to show.
func (m *Model) RefreshSync() { m.refreshSync() }

// SetSyncBackground keeps syncing while the app is out of sight, or
// stops.
func (m *Model) SetSyncBackground(on bool) {
	s := &m.Sync
	s.Background = on
	if on && s.release == nil {
		s.release = keepRunning(m.App, "Syncing your folders")
	}
	if !on && s.release != nil {
		s.release()
		s.release = nil
	}
}

// AddSyncFolder starts syncing a folder with the devices chosen.
func (m *Model) AddSyncFolder(f devsync.Folder) error {
	if m.Sync.Node == nil {
		return nil
	}
	if err := androidstore.EnsureAccess(); err != nil {
		return err
	}
	_, err := m.Sync.Node.AddFolder(f)
	m.refreshSync()
	return err
}

// AcceptSyncFolder syncs a folder a device offered into dir.
func (m *Model) AcceptSyncFolder(p devsync.Pending, dir string, mode devsync.Mode) error {
	if m.Sync.Node == nil {
		return nil
	}
	if err := androidstore.EnsureAccess(); err != nil {
		return err
	}
	err := m.Sync.Node.AcceptFolder(p, dir, mode)
	m.refreshSync()
	return err
}

// SuggestSyncPath is where a folder offered under label could go.
func SuggestSyncPath(label string) string {
	var base string
	switch runtime.GOOS {
	case "android":
		base = "/storage/emulated/0"
	case "ios":
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, "Documents")
	default:
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, "Synced")
	}
	clean := strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, r) || r < 32 {
			return '-'
		}
		return r
	}, label)
	if clean == "" {
		clean = "Synced folder"
	}
	return filepath.Join(base, clean)
}
