package phonestore

import (
	"context"
	"io/fs"
	"path"
	"strings"

	"github.com/xmhrnx/remanance/engine/fsys"
)

// folders finds or makes a volume's folders by their path.
type folders struct {
	root *fsys.Entry
	by   map[string]*fsys.Entry
}

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
