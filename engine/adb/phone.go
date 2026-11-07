package adb

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/xmhrnx/remanance/engine/appdata"
	"github.com/xmhrnx/remanance/engine/disk"
	"github.com/xmhrnx/remanance/engine/phonestore"
)

type storageDevice struct {
	*phonestore.Device
	fs *storageFS
}

// shell runs commands on a phone, as root once root has said yes.
type shell struct {
	serial string
	mu     sync.Mutex
	su     string // "" while unknown, "none" for adbd running as root, "su"
}

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

var (
	rootedMu sync.Mutex
	rooted   = map[string]bool{}
)

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
