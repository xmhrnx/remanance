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
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/xmhrnx/remanance/engine/disk"
	"github.com/xmhrnx/remanance/engine/fsys"
)

// Shell runs commands as root on the phone and reads its files.
type Shell interface {
	// Run runs a command and returns what it printed.
	Run(command string) (string, error)
	// Read fills p with the file at path from off, both multiples of
	// 512; past the file's end, it reads zeros up to the next multiple.
	Read(path string, p []byte, off int64) error
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

// Device is a phone's app databases opened for scanning.
type Device struct {
	*disk.FileSet
	sh    Shell
	sizes map[string]int64
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

// reader reads a file through the shell.
type reader struct {
	sh   Shell
	path string
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

type file struct{ r interface{ ReadAt([]byte, int64) (int, error) } }

func (f file) ReadAt(p []byte, off int64) (int, error) { return f.r.ReadAt(p, off) }
func (f file) Close() error                            { return nil }

// Volumes lists the databases, a folder for each app.
func (d *Device) Volumes(ctx context.Context, progress func(float64)) ([]*fsys.Volume, error) {
	out, err := d.sh.Run(findCommand)
	if err != nil {
		return nil, fmt.Errorf("listing the apps' data: %w", err)
	}
	progress(0.5)
	v := fsys.NewVolume("App data", 0, 0)
	v.Label = "Apps' databases"
	dirs := map[string]*fsys.Entry{"": v.Root}
	var dir func(p string) *fsys.Entry
	dir = func(p string) *fsys.Entry {
		if e, ok := dirs[p]; ok {
			return e
		}
		e := &fsys.Entry{Name: path.Base(p), IsDir: true, Health: 100}
		parent := path.Dir(p)
		if parent == "." {
			parent = ""
		}
		dir(parent).Add(e)
		dirs[p] = e
		return e
	}
	var id uint64
	for _, f := range parse(out) {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		rel := shown(f.path)
		if rel == "" || d.sizes[f.path] != 0 {
			continue
		}
		d.sizes[f.path] = f.size
		id++
		e := &fsys.Entry{ID: id, Name: path.Base(rel), Size: f.size, Modified: f.mod, Health: 100}
		e.Runs = []fsys.Run{{Offset: d.Add(f.path, f.size), Length: f.size}}
		parent := path.Dir(rel)
		if parent == "." {
			parent = ""
		}
		dir(parent).Add(e)
	}
	if id == 0 {
		return nil, fmt.Errorf("found no app databases: the phone's root shell can't read /data")
	}
	v.Size = d.Size()
	v.Sort()
	return []*fsys.Volume{v}, nil
}

// CarveRange is empty: the databases are whole files, searched row by
// row.
func (d *Device) CarveRange() (int64, int64) { return 0, 0 }

type found struct {
	path string
	size int64
	mod  time.Time
}

// parse reads find's "size mtime path" lines.
func parse(out string) []found {
	var fs []found
	for _, line := range strings.Split(out, "\n") {
		f := strings.SplitN(strings.TrimRight(line, "\r"), " ", 3)
		if len(f) != 3 {
			continue
		}
		size, err1 := strconv.ParseInt(f[0], 10, 64)
		mod, err2 := strconv.ParseInt(f[1], 10, 64)
		if err1 != nil || err2 != nil || size <= 0 || !strings.HasPrefix(f[2], "/data/") {
			continue
		}
		fs = append(fs, found{path: f[2], size: size, mod: time.Unix(mod, 0)})
	}
	return fs
}

// shown is where a database is shown: under its app's name, with the
// folder the phone keeps it in dropped.
func shown(p string) string {
	for _, place := range places {
		if rest, ok := strings.CutPrefix(p, place+"/"); ok {
			return rest
		}
	}
	return ""
}
