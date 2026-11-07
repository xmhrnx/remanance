// Package phonestore looks through an Android phone's shared storage,
// /storage/emulated/0, for what deleted photos and videos leave behind.
// The storage itself is encrypted file by file, so a deleted file's
// sectors can't be read back; what can be found is:
//
//   - the system trash, where Android 11 and later keep deleted photos
//     and videos for 30 days as hidden ".trashed-" files;
//   - copies and thumbnails apps keep in hidden folders and folders with
//     a .nomedia file, which the gallery doesn't show and which often
//     outlive the original;
//   - caches with pictures inside, such as the gallery's thumbnail
//     databases, which a deep scan searches by signature;
//   - apps' own caches in Android/data, which ADB can read: gallery,
//     camera and chat apps keep thumbnails and copies of photos there.
//
// Reading other apps' files takes Android's "All files access", on the
// phone; from a computer, ADB reads them (see engine/adb).
package phonestore

import (
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/xmhrnx/remanance/engine/disk"
)

// Root is the phone's shared storage.
const Root = "/storage/emulated/0"

// Device is the shared storage opened for scanning, as a scan.Tree: its
// files are listed when the scan starts.
type Device struct {
	*disk.FileSet
	fs         fs.FS // the storage, its paths relative to its top
	start, end int64 // what a deep scan searches: the caches
}

// found is a file worth listing or searching.
type found struct {
	rel  string // its path under the root, with "/"
	size int64
	mod  time.Time
	kind kind
}

type kind int

const (
	trashed kind = iota // in the system trash
	copied              // a media file in a hidden place
	cache               // anything else in a hidden place: searched by signature
)

// Info describes the shared storage under root as a source to scan.
func Info(root string) disk.Info {
	return disk.Info{ID: "phonestore:" + root, Path: root, Name: "This phone", Model: "Trash, hidden copies and caches",
		Bus: "Internal", Kind: disk.KindFiles}
}

// Open opens the shared storage under root.
func Open(root string) (*Device, error) {
	if _, err := os.ReadDir(root); err != nil {
		return nil, err
	}
	open := func(p string) (disk.File, error) { return os.Open(filepath.Join(root, filepath.FromSlash(p))) }
	return OpenFS(Info(root), os.DirFS(root), open), nil
}

// OpenFS opens shared storage that storage lists and open reads, by
// paths relative to its top, such as a phone's over ADB.
func OpenFS(info disk.Info, storage fs.FS, open disk.Opener) *Device {
	return &Device{FileSet: disk.NewFileSetWith(info, open), fs: storage}
}

// CarveRange is where the caches are.
func (d *Device) CarveRange() (int64, int64) { return d.start, d.end }
