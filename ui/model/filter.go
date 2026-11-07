package model

import (
	"strings"

	"github.com/xmhrnx/remanance/engine/carve"
	"github.com/xmhrnx/remanance/engine/fsys"
	"github.com/xmhrnx/remanance/locales"
)

// Show picks which files the table lists.
type Show int

const (
	ShowDeleted Show = iota // deleted, orphaned and carved files
	ShowAll
)

// Category narrows the table to one kind of file.
type Category string

const (
	CatAny       Category = ""
	CatPictures  Category = "Pictures"
	CatVideo     Category = "Video"
	CatAudio     Category = "Audio"
	CatDocuments Category = "Documents"
	CatArchives  Category = "Archives"
)

// Filter is what the table shows of the current folder.
type Filter struct {
	Show     Show
	Category Category
	Query    string
}

type rowKey struct {
	folder *Node
	filter Filter
	order  Order
}

var extCategory = map[string]Category{}

// Label names a category in the user's language.
func (c Category) Label() string { return locales.T("filter." + strings.ToLower(string(c))) }

func init() {
	for _, s := range carve.Signatures() {
		switch string(s.Category) {
		case "Pictures", "Video", "Audio", "Documents", "Archives":
			extCategory[s.Ext] = Category(s.Category)
		}
	}
	for _, e := range []string{"jpeg", "heic", "heif", "webp", "tif", "cr2", "nef", "arw", "dng"} {
		extCategory[e] = CatPictures
	}
	for _, e := range []string{"mov", "m4v", "3gp", "webm"} {
		extCategory[e] = CatVideo
	}
	for _, e := range []string{"m4a", "flac", "aac", "opus", "aiff"} {
		extCategory[e] = CatAudio
	}
	for _, e := range []string{"doc", "docx", "xls", "xlsx", "ppt", "pptx", "odt", "ods", "odp", "txt", "md", "csv", "rtf", "epub"} {
		extCategory[e] = CatDocuments
	}
	for _, e := range []string{"zip", "7z", "rar", "gz", "tar", "bz2", "xz"} {
		extCategory[e] = CatArchives
	}
}

// CategoryOf is the kind of file an entry is, from its extension.
func CategoryOf(e *fsys.Entry) Category { return extCategory[e.Ext()] }

func (f Filter) keep(e *fsys.Entry) bool {
	if !e.IsDir {
		if f.Show == ShowDeleted && e.State == fsys.Existing {
			return false
		}
		if f.Category != CatAny && CategoryOf(e) != f.Category {
			return false
		}
	} else if f.Show == ShowDeleted && e.State == fsys.Existing && !e.HasDeleted() {
		return false
	}
	return f.Query == "" || strings.Contains(strings.ToLower(e.Name), f.Query)
}

// SetFilter changes what the table shows.
func (m *Model) SetFilter(f Filter) {
	m.Filter = f
}
