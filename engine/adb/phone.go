package adb

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/xmhrnx/remanance/engine/appdata"
	"github.com/xmhrnx/remanance/engine/disk"
	"github.com/xmhrnx/remanance/engine/phonestore"
)

// ErrUnauthorized means the phone hasn't allowed this computer yet.
var ErrUnauthorized = errors.New("allow USB debugging on the phone: unlock it and tap Allow when it asks about this computer, then refresh")

// storage is where Android keeps the first user's shared storage.
const storage = "/storage/emulated/0"

// readTimeout is how long one read of a file may take.
const readTimeout = time.Minute

// StorageInfo describes a phone's shared storage as a source to scan.
func StorageInfo(d Device) disk.Info {
	info := disk.Info{ID: "adb:" + d.Serial + ":storage", Path: d.Serial, Name: d.Model,
		Model: "Trash, hidden copies and caches", Bus: "USB, through ADB", Kind: disk.KindFiles}
	if d.State == "unauthorized" {
		info.Model = "Allow USB debugging on the phone"
	}
	return info
}

// AppDataInfo describes a rooted phone's app databases.
func AppDataInfo(d Device) disk.Info {
	info := appdata.Info("adb:"+d.Serial+":appdata", d.Model)
	info.Path = d.Serial
	info.Bus = "USB, through ADB, with root"
	return info
}

// OpenStorage opens a phone's shared storage.
func OpenStorage(d Device) (disk.Device, error) {
	if d.State == "unauthorized" {
		return nil, ErrUnauthorized
	}
	if d.State != "device" {
		return nil, fmt.Errorf("the phone is %s: reconnect it", d.State)
	}
	fs := newStorageFS(d.Serial, storage)
	if _, err := fs.ReadDir("."); err != nil {
		fs.Close()
		return nil, err
	}
	sh := &shell{serial: d.Serial}
	open := func(p string) (disk.File, error) {
		return fileReader{sh: sh, path: storage + "/" + p}, nil
	}
	return &storageDevice{Device: phonestore.OpenFS(StorageInfo(d), fs, open), fs: fs}, nil
}

type storageDevice struct {
	*phonestore.Device
	fs *storageFS
}

func (s *storageDevice) Close() error {
	s.fs.Close()
	return s.Device.Close()
}

// OpenAppData opens a rooted phone's app databases.
func OpenAppData(d Device) (disk.Device, error) {
	sh := &shell{serial: d.Serial}
	if !sh.root() {
		return nil, fmt.Errorf("the phone didn't give root: allow the shell in its root manager, such as Magisk, then try again")
	}
	return appdata.Open(AppDataInfo(d), sh), nil
}

// HasSU tells whether a phone looks rooted: adb runs as root, or there
// is an su to ask, which this doesn't, since a root manager may prompt
// the phone's owner. Asked once per phone.
func HasSU(d Device) bool {
	if d.State != "device" {
		return false
	}
	rootedMu.Lock()
	r, ok := rooted[d.Serial]
	rootedMu.Unlock()
	if ok {
		return r
	}
	out, err := run(d.Serial, "id -u; command -v su", 10*time.Second)
	r = err == nil && (strings.HasPrefix(string(out), "0\n") || strings.Contains(string(out), "/su"))
	rootedMu.Lock()
	rooted[d.Serial] = r
	rootedMu.Unlock()
	return r
}

var (
	rootedMu sync.Mutex
	rooted   = map[string]bool{}
)

// shell runs commands on a phone, as root once root has said yes.
type shell struct {
	serial string
	mu     sync.Mutex
	su     string // "" while unknown, "none" for adbd running as root, "su"
}

// root finds how to run commands as root.
func (s *shell) root() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.su != "" {
		return true
	}
	if out, err := run(s.serial, "id -u", 10*time.Second); err == nil && strings.TrimSpace(string(out)) == "0" {
		s.su = "none"
		return true
	}
	// su may wait for the owner to answer a prompt on the phone.
	if out, err := run(s.serial, "su -c 'id -u'", time.Minute); err == nil && strings.TrimSpace(string(out)) == "0" {
		s.su = "su"
		return true
	}
	return false
}

// asRoot wraps a command to run as root.
func (s *shell) asRoot(cmd string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.su == "su" {
		return "su -c " + quote(cmd)
	}
	return cmd
}

// Run runs a command as root.
func (s *shell) Run(command string) (string, error) {
	out, err := run(s.serial, s.asRoot(command), 5*time.Minute)
	return string(out), err
}

// Read fills p from the file at path, with dd, which pads the end of
// the file to a whole block.
func (s *shell) Read(path string, p []byte, off int64) error {
	cmd := fmt.Sprintf("dd if=%s bs=512 skip=%d count=%d conv=noerror,sync 2>/dev/null", quote(path), off/512, len(p)/512)
	out, err := run(s.serial, s.asRoot(cmd), readTimeout)
	if err != nil {
		return err
	}
	if len(out) < len(p) {
		// Past the end of a file that shrank: zeros.
		clear(p[copy(p, out):])
		return nil
	}
	copy(p, out)
	return nil
}

// fileReader reads a file on the phone at any offset.
type fileReader struct {
	sh   *shell
	path string
}

func (f fileReader) ReadAt(p []byte, off int64) (int, error) {
	start := off &^ 511
	end := (off + int64(len(p)) + 511) &^ 511
	const most = 4 << 20
	buf := make([]byte, end-start)
	for n := int64(0); n < int64(len(buf)); n += most {
		k := min(most, int64(len(buf))-n)
		if err := f.sh.Read(f.path, buf[n:n+k], start+n); err != nil {
			return 0, err
		}
	}
	return copy(p, buf[off-start:]), nil
}

func (f fileReader) Close() error { return nil }
