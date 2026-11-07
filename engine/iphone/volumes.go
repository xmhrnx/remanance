package iphone

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/xmhrnx/remanance/engine/disk"
	"github.com/xmhrnx/remanance/engine/fsys"
	"github.com/xmhrnx/remanance/engine/iosbackup"
)

// Volumes walks the media folder and lists the camera roll, with
// Recently Deleted marked, the previews of photos that are gone, and the
// Photos library's databases.
func (m *Media) Volumes(ctx context.Context, progress func(float64)) ([]*fsys.Volume, error) {
	top, err := m.a.list("/")
	if err != nil {
		return nil, fmt.Errorf("reading the iPhone: %w", err)
	}
	var files []file
	for i, name := range top {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		m.walk(ctx, name, &files)
		progress(float64(i+1) / float64(len(top)) * 0.9)
	}

	var roll, previews, library, caches []file
	have := map[string]bool{} // the camera roll's files, lowercase
	for _, f := range files {
		if strings.HasPrefix(f.path, "DCIM/") && isMedia(f.path) {
			roll = append(roll, f)
			have[strings.ToLower(f.path)] = true
		}
	}
	best := map[string]file{} // the largest preview of each photo that's gone
	for _, f := range files {
		switch {
		case strings.HasPrefix(f.path, "DCIM/"):
		case !strings.HasPrefix(f.path, "PhotoData/"):
		case isDB(f.path):
			library = append(library, f)
		case isMedia(f.path):
			orig := original(f.path)
			if orig == "" || have[strings.ToLower(orig)] {
				continue
			}
			if b, ok := best[orig]; !ok || f.size > b.size {
				best[orig] = f
			}
		case f.size >= 4<<10:
			caches = append(caches, f)
		}
	}
	for _, f := range best {
		previews = append(previews, f)
	}
	sort.Slice(previews, func(i, j int) bool { return previews[i].path < previews[j].path })

	// The caches go first, so a deep scan searches one stretch.
	m.start = m.Size()
	for _, f := range caches {
		m.Add("/"+f.path, f.size)
	}
	m.end = m.Size()

	var id uint64
	add := func(dirs *folders, at string, f file) *fsys.Entry {
		id++
		e := &fsys.Entry{ID: id, Name: path.Base(at), Size: f.size, Modified: f.mod, Health: 100}
		e.Runs = []fsys.Run{{Offset: m.Add("/"+f.path, f.size), Length: f.size}}
		dirs.at(path.Dir(at)).Add(e)
		return e
	}

	rollVol := fsys.NewVolume("iPhone", 0, 0)
	rollVol.Label = "Camera roll"
	rollDirs := &folders{rollVol.Root, map[string]*fsys.Entry{}}
	byPath := map[string]*fsys.Entry{}
	for _, f := range roll {
		byPath[f.path] = add(rollDirs, strings.TrimPrefix(f.path, "DCIM/"), f)
	}

	prevVol := fsys.NewVolume("iPhone", 0, 0)
	prevVol.Label = "Previews of deleted photos"
	prevDirs := &folders{prevVol.Root, map[string]*fsys.Entry{}}
	for _, f := range previews {
		orig := original(f.path)
		// Named after the photo, with the preview's own extension.
		name := strings.TrimSuffix(path.Base(orig), path.Ext(orig)) + strings.ToLower(path.Ext(f.path))
		e := add(prevDirs, path.Join(strings.TrimPrefix(path.Dir(orig), "DCIM/"), name), f)
		e.State = fsys.Deleted
		e.Note = "A preview iOS kept of " + path.Base(orig) + ", which is no longer on the iPhone"
	}

	libVol := fsys.NewVolume("iPhone", 0, 0)
	libVol.Label = "Photos library"
	libDirs := &folders{libVol.Root, map[string]*fsys.Entry{}}
	var photosDB *fsys.Entry
	for _, f := range library {
		e := add(libDirs, strings.TrimPrefix(f.path, "PhotoData/"), f)
		if f.path == "PhotoData/Photos.sqlite" {
			photosDB = e
		}
	}
	progress(0.95)
	if photosDB != nil {
		iosbackup.MarkTrashed(disk.NewSection(m.FileSet, photosDB.Runs[0].Offset, photosDB.Size), photosDB.Size, byPath)
	}

	var out []*fsys.Volume
	for _, v := range []*fsys.Volume{rollVol, prevVol, libVol} {
		if len(v.Root.Children) == 0 && v != rollVol {
			continue
		}
		v.Size = m.Size()
		v.Sort()
		out = append(out, v)
	}
	progress(1)
	return out, nil
}

// walk lists the files in the folder rel and below.
func (m *Media) walk(ctx context.Context, rel string, out *[]file) {
	if ctx.Err() != nil {
		return
	}
	info, err := m.a.stat("/" + rel)
	if err != nil || info.link {
		return
	}
	if !info.dir {
		if info.size > 0 {
			*out = append(*out, file{path: rel, size: info.size, mod: info.mod})
		}
		return
	}
	names, err := m.a.list("/" + rel)
	if err != nil {
		return
	}
	for _, n := range names {
		m.walk(ctx, rel+"/"+n, out)
	}
}
