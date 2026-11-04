// Package cleanup finds files a computer or phone can do without, to
// free space: temporary files, caches apps rebuild, crash reports and
// logs, the trash, and installers and old files left in Downloads.
//
// What's safe to delete is found in places each system keeps for it
// (see sources_*.go). Caches and temporary files are deleted for good,
// since the apps make them again; a download is moved to the trash, as
// it may be the only copy.
package cleanup

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/xmhrnx/remanance/engine/trash"
)

// Kind is a sort of file that can be cleaned.
type Kind int

const (
	Temp         Kind = iota // temporary files
	Caches                   // caches apps rebuild
	BuildCaches              // developer tools' caches
	Logs                     // crash reports and logs
	Trash                    // the trash
	Installers               // installers in Downloads
	OldDownloads             // downloads untouched for a long while
	kinds
)

// Label names a kind.
func (k Kind) Label() string {
	return [...]string{"Temporary files", "App caches", "Developer caches", "Crash reports and logs",
		"Trash", "Installers", "Old downloads"}[k]
}

// Safe says whether a kind can be cleaned without the user looking
// through it first: what's in it is made again or isn't needed.
func (k Kind) Safe() bool {
	switch k {
	case Temp, Caches, Logs:
		return true
	}
	return false
}

// ToTrash says whether a kind's files go to the trash, rather than
// being deleted, where there's a trash.
func (k Kind) ToTrash() bool { return k == Installers || k == OldDownloads }

// Kinds lists the kinds in the order they're shown.
func Kinds() []Kind {
	out := make([]Kind, kinds)
	for i := range out {
		out[i] = Kind(i)
	}
	return out
}

// OldAge is how long a download goes untouched before it's counted old.
const OldAge = 90 * 24 * time.Hour

// inUse is how recently a temporary file may have changed and still be
// in use: newer ones are left alone.
const inUse = 24 * time.Hour

// Item is a file or folder to clean.
type Item struct {
	Kind     Kind
	Path     string
	Name     string // what to call it, when its path doesn't say
	Size     int64
	Files    int
	Modified time.Time // the newest file in it
	// Keep keeps the folder itself, emptying it: a place an app or the
	// system expects to find.
	Keep bool
}

// Title is what to call the item.
func (it Item) Title() string {
	if it.Name != "" {
		return it.Name
	}
	return filepath.Base(it.Path)
}

// Group is the items of one kind, largest first.
type Group struct {
	Kind  Kind
	Items []Item
}

// Size is the space the group takes.
func (g Group) Size() int64 {
	var n int64
	for _, it := range g.Items {
		n += it.Size
	}
	return n
}

// Files counts the group's files.
func (g Group) Files() int {
	n := 0
	for _, it := range g.Items {
		n += it.Files
	}
	return n
}

// Source is a place to look.
type Source struct {
	Kind Kind
	Dir  string
	// Name is what to call the place, such as the app whose cache it
	// is. Empty for a place whose contents are listed one by one.
	Name string
	// Each lists what's in Dir as items, rather than Dir as one.
	Each bool
}

// places makes a source of each folder matching a pattern, such as a
// browser's cache in each of its profiles.
func places(kind Kind, name string, patterns ...string) []Source {
	var out []Source
	for _, pat := range patterns {
		dirs, _ := filepath.Glob(pat)
		for _, d := range dirs {
			out = append(out, Source{Kind: kind, Dir: d, Name: name})
		}
	}
	return out
}

// Progress says how a search is going.
type Progress struct {
	Kind  Kind
	Files int
	Bytes int64
}

// Find looks through the places this system keeps files it can do
// without.
func Find(ctx context.Context, progress func(Progress)) ([]Group, error) {
	return FindIn(ctx, Sources(), time.Now(), progress)
}

// FindIn looks through the sources given, as of now.
func FindIn(ctx context.Context, sources []Source, now time.Time, progress func(Progress)) ([]Group, error) {
	var p Progress
	tick := time.Now()
	report := func(it Item) {
		p.Files += it.Files
		p.Bytes += it.Size
		if progress != nil && time.Since(tick) > 100*time.Millisecond {
			tick = time.Now()
			progress(p)
		}
	}
	byKind := map[Kind][]Item{}
	seen := map[string]bool{}
	add := func(it Item) {
		if it.Size == 0 || seen[it.Path] {
			return
		}
		seen[it.Path] = true
		byKind[it.Kind] = append(byKind[it.Kind], it)
		report(it)
	}
	for _, s := range sources {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		p.Kind = s.Kind
		if s.Kind == Trash {
			if u, err := trash.Size(); err == nil && u.Bytes > 0 {
				byKind[Trash] = append(byKind[Trash], Item{Kind: Trash, Path: trashPath, Name: "Trash", Size: u.Bytes, Files: u.Files})
			}
			continue
		}
		if !s.Each {
			it, err := measure(ctx, s.Dir)
			if err != nil {
				continue
			}
			it.Kind, it.Name, it.Keep = s.Kind, s.Name, true
			add(it)
			continue
		}
		es, err := os.ReadDir(s.Dir)
		if err != nil {
			continue
		}
		for _, e := range es {
			if e.Type()&fs.ModeSymlink != 0 {
				continue
			}
			// Others' temporary files can't be deleted, and aren't the
			// user's to delete.
			if s.Kind == Temp && !mine(filepath.Join(s.Dir, e.Name())) {
				continue
			}
			it, err := measure(ctx, filepath.Join(s.Dir, e.Name()))
			if err != nil {
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				continue
			}
			it.Kind = s.Kind
			if s.Kind == Caches && buildCache(e.Name()) {
				it.Kind = BuildCaches
			}
			if s.Kind == OldDownloads {
				switch {
				case installer(e.Name()):
					it.Kind = Installers
				case now.Sub(it.Modified) < OldAge:
					continue
				}
			}
			if s.Kind == Temp && now.Sub(it.Modified) < inUse {
				continue
			}
			add(it)
		}
	}
	if p.Files > 0 && progress != nil {
		progress(p)
	}
	var out []Group
	for _, k := range Kinds() {
		items := byKind[k]
		if len(items) == 0 {
			continue
		}
		sort.Slice(items, func(i, j int) bool { return items[i].Size > items[j].Size })
		out = append(out, Group{Kind: k, Items: items})
	}
	return out, nil
}

// trashPath stands for the trash, which has no one path everywhere.
const trashPath = "trash:"

// measure adds up a file, or the files in a folder, and finds the
// newest change in it.
func measure(ctx context.Context, path string) (Item, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return Item{}, err
	}
	it := Item{Path: path, Modified: info.ModTime()}
	if !info.IsDir() {
		if info.Mode().IsRegular() {
			it.Size, it.Files = info.Size(), 1
		}
		return it, nil
	}
	err = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable: left out
		}
		if it.Files%512 == 0 && ctx.Err() != nil {
			return ctx.Err()
		}
		if !d.Type().IsRegular() {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return nil
		}
		it.Size += fi.Size()
		it.Files++
		if fi.ModTime().After(it.Modified) {
			it.Modified = fi.ModTime()
		}
		return nil
	})
	return it, err
}

// buildCaches are the folders developer tools keep their caches in,
// inside a system's cache folder. Clearing them is safe but makes the
// next build slow, so they're shown apart.
var buildCaches = map[string]bool{
	"go-build": true, "gopls": true, "pip": true, "pypoetry": true, "uv": true, "npm": true,
	"npm-cache": true, "yarn": true, "pnpm": true, "node-gyp": true, "ms-playwright": true,
	"cargo": true, "bazel": true, "gradle": true, "maven": true, "coursier": true,
	"jetbrains": true, "huggingface": true, "torch": true, "electron": true, "electron-builder": true,
	"homebrew": true, "cypress": true, "deno": true, "bun": true, "composer": true,
}

func buildCache(name string) bool { return buildCaches[strings.ToLower(name)] }

// installerExts are the kinds of file that install an app, which are
// rarely needed once it is.
var installerExts = map[string]bool{
	".exe": true, ".msi": true, ".msix": true, ".appx": true, ".dmg": true, ".pkg": true,
	".apk": true, ".xapk": true, ".deb": true, ".rpm": true, ".appimage": true, ".flatpakref": true,
}

func installer(name string) bool { return installerExts[strings.ToLower(filepath.Ext(name))] }

// Result says what cleaning did.
type Result struct {
	Freed   int64 // deleted for good
	Trashed int64 // moved to the trash
	Files   int
	// Left counts files that couldn't be deleted, mostly because an app
	// has them open.
	Left int
}

// Clean deletes the items, or moves them to the trash for the kinds
// that go there, and says how much space it freed. A file that can't be
// deleted is left and counted; only a failure to clean anything is an
// error.
func Clean(ctx context.Context, items []Item, progress func(done, of int)) (Result, error) {
	var r Result
	var first error
	for i, it := range items {
		if err := ctx.Err(); err != nil {
			return r, err
		}
		if progress != nil {
			progress(i, len(items))
		}
		switch {
		case it.Kind == Trash:
			if err := trash.Empty(); err != nil {
				first = errors.Join(first, err)
				continue
			}
			r.Freed += it.Size
			r.Files += it.Files
		case it.Kind.ToTrash():
			if err := trash.Remove(it.Path, true); err != nil {
				first = errors.Join(first, err)
				r.Left += it.Files
				continue
			}
			if trash.Supported {
				r.Trashed += it.Size
			} else {
				r.Freed += it.Size
			}
			r.Files += it.Files
		default:
			freed, files, left := remove(it.Path, it.Keep)
			r.Freed += freed
			r.Files += files
			r.Left += left
		}
	}
	if progress != nil {
		progress(len(items), len(items))
	}
	if r.Files == 0 && first != nil {
		return r, first
	}
	return r, nil
}

// remove deletes a file, or the files in a folder one by one so those in
// use are skipped rather than stopping it, then the folders left empty.
func remove(path string, keep bool) (freed int64, files, left int) {
	info, err := os.Lstat(path)
	if err != nil {
		return 0, 0, 0
	}
	if !info.IsDir() {
		if os.Remove(path) != nil {
			return 0, 0, 1
		}
		return info.Size(), 1, 0
	}
	var dirs []string
	filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			dirs = append(dirs, p)
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return nil
		}
		if os.Remove(p) != nil {
			left++
			return nil
		}
		freed += fi.Size()
		if fi.Mode().IsRegular() {
			files++
		}
		return nil
	})
	// Deepest first, so a folder's folders are gone before it.
	for i := len(dirs) - 1; i >= 0; i-- {
		if keep && dirs[i] == path {
			continue
		}
		os.Remove(dirs[i]) // fails, as it should, while something's left in it
	}
	return freed, files, left
}

// Space is how much a drive holds and has free.
type Space struct {
	Free, Total int64
}

// Free says how full the drive holding path is.
func Free(path string) (Space, error) { return free(path) }
