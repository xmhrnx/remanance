package cleanup

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/xmhrnx/remanance/engine/trash"
)

// OldAge is how long a download goes untouched before it's counted old.
const OldAge = 90 * 24 * time.Hour

// inUse is how recently a temporary file may have changed and still be
// in use: newer ones are left alone.
const inUse = 24 * time.Hour

// trashPath stands for the trash, which has no one path everywhere.
const trashPath = "trash:"

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
