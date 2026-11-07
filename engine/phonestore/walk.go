package phonestore

import (
	"context"
	"io/fs"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/xmhrnx/remanance/engine/fsys"
)

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
