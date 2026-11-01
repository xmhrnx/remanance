package devsync

import (
	"time"
)

// Status is what the app shows about syncing.
type Status struct {
	Devices []DeviceStatus
	Folders []FolderStatus
	Pending []Pending
}

// DeviceStatus is a device and whether it's connected.
type DeviceStatus struct {
	Device
	Connected bool
	Addr      string // where it's connected from
}

// FolderStatus is a folder and how its syncing goes.
type FolderStatus struct {
	Folder
	State     FolderState
	Files     int   // here, not counting deleted ones
	Bytes     int64 // their size
	Need      int   // files to take from the other devices
	NeedBytes int64
	Online    int // devices it's shared with that are connected
	LastSync  time.Time
	Err       string
}

// FolderState is what a folder is doing.
type FolderState int

const (
	StateIdle FolderState = iota
	StateScanning
	StateSyncing
	StatePaused
	StateError
	StateWaiting // no device it's shared with is connected
)

// Label names a state.
func (s FolderState) Label() string {
	return [...]string{"Up to date", "Looking for changes", "Syncing", "Paused", "Problem", "Waiting for devices"}[s]
}

// Status reports the devices, folders and requests.
func (n *Node) Status() Status {
	n.mu.Lock()
	defer n.mu.Unlock()
	var s Status
	for _, d := range n.cfg.Devices {
		ds := DeviceStatus{Device: d}
		if c := n.conns[d.ID]; c != nil {
			ds.Connected, ds.Addr = true, c.addr
		}
		s.Devices = append(s.Devices, ds)
	}
	for _, f := range n.cfg.Folders {
		fs := FolderStatus{Folder: f}
		if ix := n.idx[f.ID]; ix != nil {
			for _, file := range ix.Files {
				if !file.Deleted {
					fs.Files++
					fs.Bytes += file.Size
				}
			}
		}
		for _, d := range f.Devices {
			if c := n.conns[d]; c != nil && c.folders[f.ID] {
				fs.Online++
			}
		}
		st := n.state[f.ID]
		if st != nil {
			fs.LastSync, fs.Err = st.lastSync, st.err
		}
		if f.Mode != SendOnly {
			fs.Need, fs.NeedBytes = n.needLocked(f)
		}
		switch {
		case f.Paused:
			fs.State = StatePaused
		case st != nil && st.scanning:
			fs.State = StateScanning
		case st != nil && st.pulling:
			fs.State = StateSyncing
		case fs.Err != "":
			fs.State = StateError
		case fs.Online == 0 && len(f.Devices) > 0:
			fs.State = StateWaiting
		case fs.Need > 0:
			fs.State = StateSyncing
		}
		s.Folders = append(s.Folders, fs)
	}
	s.Pending = append(s.Pending, n.cfg.Pending...)
	return s
}

// needLocked counts the files a folder has yet to take. n.mu is held.
func (n *Node) needLocked(f Folder) (int, int64) {
	ix := n.idx[f.ID]
	if ix == nil {
		return 0, 0
	}
	seen := map[string]bool{}
	count, bytes := 0, int64(0)
	for dev, files := range n.remote[f.ID] {
		if c := n.conns[dev]; c == nil || n.shared(f.ID, c) == nil {
			continue
		}
		for name, rf := range files {
			if seen[name] {
				continue
			}
			local, have := ix.Files[name]
			if (!have && !rf.Deleted) || (have && rf.Version.compare(local.Version) == newer) {
				seen[name] = true
				count++
				bytes += rf.Size
			}
		}
	}
	return count, bytes
}
