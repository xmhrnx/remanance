// Package appdata reads the databases apps keep in their own folders on
// an Android phone, /data/data and /data/user_de: texts and picture
// messages, the call log, contacts, chats. Only root may read them, and
// that is where recovery tools find deleted messages: a database keeps a
// deleted row until it needs the space (see engine/records, which a scan
// runs on every database listed here).
//
// The files are read through a root shell, the phone's own su or one
// over ADB, as the shell sees them: decrypted, since the phone is
// unlocked.
package appdata

import (
	"fmt"
	"time"

	"github.com/xmhrnx/remanance/engine/disk"
)

type file struct {
	r interface {
		ReadAt([]byte, int64) (int, error)
	}
}

type found struct {
	path string
	size int64
	mod  time.Time
}

// Shell runs commands as root on the phone and reads its files.
type Shell interface {
	// Run runs a command and returns what it printed.
	Run(command string) (string, error)
	// Read fills p with the file at path from off, both multiples of
	// 512; past the file's end, it reads zeros up to the next multiple.
	Read(path string, p []byte, off int64) error
}

// Device is a phone's app databases opened for scanning.
type Device struct {
	*disk.FileSet
	sh    Shell
	sizes map[string]int64
}

// reader reads a file through the shell.
type reader struct {
	sh   Shell
	path string
}

// places are the folders apps keep their data in, for the phone's first
// user: credential-encrypted, then device-encrypted, where the phone
// keeps texts.
var places = []string{"/data/data", "/data/user_de/0"}

// findCommand lists the databases with their sizes and dates.
const findCommand = `find /data/data /data/user_de/0 -type f \( -name '*.db' -o -name '*.db-wal' -o -name '*.sqlite' ` +
	`-o -name '*.sqlite-wal' -o -name '*.sqlite3' -o -name '*.sqlitedb' -o -name '*.storedata' \) -size +0 ` +
	`-exec stat -c '%s %Y %n' {} + 2>/dev/null`

// Info describes the app data of a phone, by its ID.
func Info(id, phone string) disk.Info {
	return disk.Info{ID: id, Path: id, Name: phone, Model: "Apps' messages, calls and chats",
		Bus: "App data, with root", Kind: disk.KindFiles}
}

// Open opens the app data a root shell reads.
func Open(info disk.Info, sh Shell) *Device {
	d := &Device{sh: sh, sizes: map[string]int64{}}
	d.FileSet = disk.NewFileSetWith(info, func(p string) (disk.File, error) {
		size, ok := d.sizes[p]
		if !ok {
			return nil, fmt.Errorf("no file %s", p)
		}
		return file{disk.Aligned(reader{sh, p}, 512, size)}, nil
	})
	return d
}

func (r reader) ReadAt(p []byte, off int64) (int, error) {
	// Long reads go in pieces, so one never holds the shell long.
	const most = 4 << 20
	for n := 0; n < len(p); n += most {
		k := min(most, len(p)-n)
		if err := r.sh.Read(r.path, p[n:n+k], off+int64(n)); err != nil {
			return n, err
		}
	}
	return len(p), nil
}

func (f file) ReadAt(p []byte, off int64) (int, error) { return f.r.ReadAt(p, off) }

func (f file) Close() error { return nil }

// CarveRange is empty: the databases are whole files, searched row by
// row.
func (d *Device) CarveRange() (int64, int64) { return 0, 0 }
