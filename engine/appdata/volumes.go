package appdata

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/xmhrnx/remanance/engine/fsys"
)

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
