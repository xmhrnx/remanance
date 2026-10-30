package model

import (
	"sort"
	"strings"

	"github.com/mehran9675/remanence/engine/carve"
	"github.com/mehran9675/remanence/engine/fsys"
	"github.com/mehran9675/remanence/engine/recovery"
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
}

var extCategory = map[string]Category{}

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

// maxSearchRows caps a search, which walks every folder below.
const maxSearchRows = 50000

// Rows lists what the table shows: the current folder's folders and
// files, or with a query, every match below it.
func (m *Model) Rows() []*fsys.Entry {
	key := rowKey{m.Folder, m.Filter}
	if key == m.rowKey && m.rowCache != nil {
		return m.rowCache
	}
	m.rowKey = key
	m.rowCache = m.rowCache[:0]
	if m.Folder == nil {
		return m.rowCache
	}
	f := m.Filter
	f.Query = strings.ToLower(strings.TrimSpace(f.Query))
	if f.Query != "" || f.Category != CatAny {
		m.Folder.Entry.Walk(func(e *fsys.Entry) bool {
			if e != m.Folder.Entry && !e.IsDir && f.keep(e) && len(m.rowCache) < maxSearchRows {
				m.rowCache = append(m.rowCache, e)
			}
			return len(m.rowCache) < maxSearchRows
		})
		sort.SliceStable(m.rowCache, func(i, j int) bool { return m.rowCache[i].Name < m.rowCache[j].Name })
		return m.rowCache
	}
	for _, c := range m.Folder.Entry.Children {
		if f.keep(c) {
			m.rowCache = append(m.rowCache, c)
		}
	}
	return m.rowCache
}

// Searching reports whether the table lists matches from every folder
// below, rather than the folder's own contents.
func (m *Model) Searching() bool {
	return strings.TrimSpace(m.Filter.Query) != "" || m.Filter.Category != CatAny
}

// SetFilter changes what the table shows.
func (m *Model) SetFilter(f Filter) {
	m.Filter = f
}

// OpenFolder shows a folder from the table or the tree.
func (m *Model) OpenFolder(n *Node) {
	m.Folder, m.Preview = n, nil
}

// OpenEntry opens a folder listed in the table.
func (m *Model) OpenEntry(e *fsys.Entry) {
	if m.Folder == nil || !e.IsDir {
		return
	}
	if c := m.Folder.Child(e); c != nil {
		m.OpenFolder(c)
	}
}

func bytes(n int64) string { return recovery.FormatBytes(n) }
