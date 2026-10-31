// Package iphone reads an iPhone or iPad connected by USB, through
// Apple's USB service (usbmuxd, which iTunes or Apple Devices installs on
// Windows): after the phone trusts this computer, its media folder,
// where the camera roll and the Photos library are, is read directly.
//
// The phone's storage is encrypted file by file, and a deleted file's
// key is erased with it, so its sectors can't be read back from any
// computer. What a scan of the media folder finds is:
//
//   - photos and videos in Recently Deleted, which iOS keeps for 30
//     days, marked from the Photos library;
//   - previews of deleted photos: iOS keeps a thumbnail of each photo
//     in PhotoData, and it often outlives the photo;
//   - deleted rows in the Photos library, which name deleted photos and
//     when they were taken (see engine/records);
//   - pictures inside the thumbnail caches, which a deep scan searches
//     by signature.
package iphone

import (
	"context"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/xmhrnx/remanance/engine/disk"
	"github.com/xmhrnx/remanance/engine/fsys"
	"github.com/xmhrnx/remanance/engine/iosbackup"
	"github.com/xmhrnx/remanance/engine/plist"
)

// Phone is an iPhone or iPad connected by USB.
type Phone struct {
	UDID    string
	Name    string // "Sam's iPhone"
	Product string // "iPhone15,2"
	dev     device
}

// Connected lists the iPhones and iPads connected by USB.
func Connected() ([]Phone, error) {
	devs, err := devices()
	if err != nil {
		return nil, err
	}
	var out []Phone
	for _, d := range devs {
		p := Phone{UDID: d.udid, Name: "iPhone", dev: d}
		if l, err := openLockdown(d); err == nil {
			if n := plist.String(l.value("", "DeviceName")); n != "" {
				p.Name = n
			}
			p.Product = plist.String(l.value("", "ProductType"))
			l.Close()
		}
		out = append(out, p)
	}
	return out, nil
}

// MediaInfo describes a phone's media folder as a source to scan.
func MediaInfo(p Phone) disk.Info {
	return disk.Info{ID: "iphone:" + p.UDID, Path: p.UDID, Name: p.Name,
		Model: "Recently Deleted, previews of deleted photos and caches", Bus: "USB", Kind: disk.KindFiles}
}

// trustTimeout is how long the phone's owner has to tap Trust.
const trustTimeout = 2 * time.Minute

// OpenMedia opens a phone's media folder. The first time, the phone asks
// its owner to trust this computer, and this waits for the answer.
func OpenMedia(p Phone) (*Media, error) {
	l, err := openLockdown(p.dev)
	if err != nil {
		return nil, err
	}
	defer l.Close()
	ctx, cancel := context.WithTimeout(context.Background(), trustTimeout)
	defer cancel()
	if err := l.pair(ctx, func() {}); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("the iPhone wasn't trusted in time: unlock it, tap Trust, enter its passcode and try again")
		}
		return nil, err
	}
	if err := l.startSession(); err != nil {
		return nil, err
	}
	c, err := l.startService("com.apple.afc", false)
	if err != nil {
		return nil, err
	}
	a := &afc{rw: c}
	m := &Media{a: a}
	m.FileSet = disk.NewFileSetWith(MediaInfo(p), func(p string) (disk.File, error) { return a.open(p) })
	return m, nil
}

// Media is a phone's media folder opened for scanning, as a scan.Tree.
type Media struct {
	*disk.FileSet
	a          *afc
	start, end int64 // what a deep scan searches: the caches
}

func (m *Media) Close() error {
	err := m.FileSet.Close()
	m.a.rw.Close()
	return err
}

// CarveRange is where the caches are.
func (m *Media) CarveRange() (int64, int64) { return m.start, m.end }

// file is a file in the media folder.
type file struct {
	path string // under the media folder, without a leading "/"
	size int64
	mod  time.Time
}

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

// original is the photo a preview in PhotoData shows, by its path in the
// media folder: iOS keeps them as
// PhotoData/Thumbnails/V2/DCIM/100APPLE/IMG_0001.HEIC/5005.JPG, and
// before iOS 9 as PhotoData/Metadata/DCIM/100APPLE/IMG_0001.JPG.
func original(p string) string {
	if i := strings.Index(p, "/DCIM/"); i >= 0 {
		rest := p[i+1:]
		if strings.HasPrefix(p, "PhotoData/Thumbnails/") {
			// The preview is a file in a folder named after the photo.
			if d := path.Dir(rest); isMedia(d) {
				return d
			}
			return ""
		}
		return rest
	}
	return ""
}

func ext(p string) string { return strings.TrimPrefix(strings.ToLower(path.Ext(p)), ".") }

func isMedia(p string) bool { return media[ext(p)] }

func isDB(p string) bool {
	switch ext(p) {
	case "sqlite", "sqlite-wal", "db", "db-wal", "sqlitedb":
		return true
	}
	return false
}

// media are the extensions of photos and videos.
var media = map[string]bool{}

func init() {
	for _, e := range strings.Fields("jpg jpeg png gif heic heif dng tif tiff webp mov mp4 m4v") {
		media[e] = true
	}
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
