package iphone

import (
	"path"
	"strings"

	"github.com/xmhrnx/remanance/engine/fsys"
)

// folders finds or makes a volume's folders by their path.
type folders struct {
	root *fsys.Entry
	by   map[string]*fsys.Entry
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
