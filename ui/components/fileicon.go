package components

import (
	"path/filepath"

	"github.com/mehran9675/giode"
	"github.com/xmhrnx/remanance/engine/fsys"
	"github.com/xmhrnx/remanance/ui/icons"
	"github.com/xmhrnx/remanance/ui/model"
)

// fileKind is how a kind of file is drawn: an icon and its color.
type fileKind struct{ icon, color string }

var kinds = map[model.Category]fileKind{
	model.CatPictures:  {"picture", "#E58FB0"},
	model.CatVideo:     {"video", "#A98BF0"},
	model.CatAudio:     {"music", "#5CC8D6"},
	model.CatDocuments: {"document", "#7CB7FF"},
	model.CatArchives:  {"archive", "#E8B45A"},
}

// FileIcon draws a file or folder's icon in a small tinted tile, the
// color telling kinds of files apart at a glance.
func FileIcon(e *fsys.Entry, size int) giode.Element {
	k := fileKind{"file", "theme.muted"}
	switch {
	case e.IsDir:
		k = fileKind{"folder", "theme.primary"}
	case e.Ext() == "db" || e.Ext() == "sqlite":
		k = fileKind{"database", "#8BD17C"}
	default:
		if c, ok := kinds[model.CategoryOf(e)]; ok {
			k = c
		}
	}
	return icons.Icon(k.icon, size, k.color)
}

// FileIconNamed draws the icon for a file on disk, by its name.
func FileIconNamed(name string, size int) giode.Element {
	return FileIcon(&fsys.Entry{Name: filepath.Base(name)}, size)
}
