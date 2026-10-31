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
	"context"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/xmhrnx/remanance/engine/disk"
	"github.com/xmhrnx/remanance/engine/fsys"
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

// Volumes walks the storage and lists the trash, then the hidden copies.
// A deep scan then searches the caches.
func (d *Device) Volumes(ctx context.Context, progress func(float64)) ([]*fsys.Volume, error) {
	top, err := fs.ReadDir(d.fs, ".")
	if err != nil {
		return nil, err
	}
	var all []found
	for i, e := range top {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if e.IsDir() {
			d.walk(ctx, e.Name(), strings.HasPrefix(e.Name(), "."), &all)
		}
		progress(float64(i+1) / float64(len(top)))
	}

	// The caches go first, so a deep scan searches one stretch.
	for _, f := range all {
		if f.kind == cache {
			d.Add(f.rel, f.size)
		}
	}
	d.end = d.Size()

	trash := fsys.NewVolume("Phone storage", 0, 0)
	trash.Label = "Trash"
	copies := fsys.NewVolume("Phone storage", 0, 0)
	copies.Label = "Hidden copies"
	trashDirs := &folders{trash.Root, map[string]*fsys.Entry{}}
	copyDirs := &folders{copies.Root, map[string]*fsys.Entry{}}
	var id uint64
	for _, f := range all {
		if f.kind == cache {
			continue
		}
		id++
		e := &fsys.Entry{ID: id, Size: f.size, Modified: f.mod, Health: 100}
		e.Runs = []fsys.Run{{Offset: d.Add(f.rel, f.size), Length: f.size}}
		dir, name := path.Split(f.rel)
		dirs := copyDirs
		if f.kind == trashed {
			dirs = trashDirs
			e.Name, e.State, e.Note = trashName(name)
		} else {
			e.Name, e.State = name, fsys.Orphan
			e.Note = "A copy an app kept out of the gallery; the original may be gone"
		}
		dirs.at(strings.TrimSuffix(dir, "/")).Add(e)
	}
	trash.Size, copies.Size = d.Size(), d.Size()
	trash.Sort()
	copies.Sort()
	out := []*fsys.Volume{trash}
	if len(copies.Root.Children) > 0 {
		out = append(out, copies)
	}
	return out, nil
}

// CarveRange is where the caches are.
func (d *Device) CarveRange() (int64, int64) { return d.start, d.end }

// walk lists what's worth finding in the folder rel and below. hidden
// says the folder is out of the gallery's sight.
func (d *Device) walk(ctx context.Context, rel string, hidden bool, out *[]found) {
	if skip[rel] || ctx.Err() != nil {
		return
	}
	entries, err := fs.ReadDir(d.fs, rel)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.Name() == ".nomedia" {
			hidden = true
		}
	}
	for _, e := range entries {
		name := e.Name()
		p := rel + "/" + name
		if e.IsDir() {
			d.walk(ctx, p, hidden || strings.HasPrefix(name, ".") || cacheDir[strings.ToLower(name)], out)
			continue
		}
		if !e.Type().IsRegular() {
			continue
		}
		info, err := e.Info()
		if err != nil || info.Size() == 0 {
			continue
		}
		f := found{rel: p, size: info.Size(), mod: info.ModTime()}
		switch {
		case strings.HasPrefix(name, ".trashed-"):
			f.kind = trashed
		case strings.HasPrefix(name, ".pending-"):
			continue // a file still being written
		case !hidden && !strings.HasPrefix(name, "."):
			continue // in plain sight: not lost
		case media[strings.TrimPrefix(strings.ToLower(path.Ext(name)), ".")]:
			f.kind = copied
		case info.Size() >= 4<<10 && info.Size() <= 4<<30:
			f.kind = cache
		default:
			continue
		}
		*out = append(*out, f)
	}
}

// trashName reads ".trashed-<when it expires>-<name>".
func trashName(file string) (name string, state fsys.State, note string) {
	rest := strings.TrimPrefix(file, ".trashed-")
	note = "In the trash"
	if ts, n, ok := strings.Cut(rest, "-"); ok {
		if secs, err := strconv.ParseInt(ts, 10, 64); err == nil && n != "" {
			rest = n
			note += "; Android erases it for good on " + time.Unix(secs, 0).Local().Format("Jan 2, 2006")
		}
	}
	return rest, fsys.Deleted, note
}

// folders finds or makes a volume's folders by their path.
type folders struct {
	root *fsys.Entry
	by   map[string]*fsys.Entry
}

func (f *folders) at(rel string) *fsys.Entry {
	if rel == "" || rel == "." {
		return f.root
	}
	if e, ok := f.by[rel]; ok {
		return e
	}
	e := &fsys.Entry{Name: path.Base(rel), IsDir: true, Health: 100}
	f.at(path.Dir(rel)).Add(e)
	f.by[rel] = e
	return e
}

// skip are folders not worth walking: they hold app packages. Apps'
// own folders, Android/data, are walked: Android denies them to other
// apps, so on the phone the walk passes them by, but over ADB they can
// be read, and gallery and chat apps keep thumbnails and copies of
// photos there that outlive the originals.
var skip = map[string]bool{"Android/obb": true}

// cacheDir names folders apps keep caches in.
var cacheDir = map[string]bool{"cache": true, "caches": true, "thumbnails": true, "thumbs": true}

// media are the extensions of photos, videos and recordings.
var media = map[string]bool{}

func init() {
	for _, e := range strings.Fields("jpg jpeg png gif webp heic heif bmp dng raw " +
		"mp4 m4v mov 3gp 3g2 mkv webm avi " +
		"mp3 m4a aac amr ogg opus wav flac") {
		media[e] = true
	}
}
