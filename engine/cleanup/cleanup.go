// Package cleanup finds files a computer or phone can do without, to
// free space: temporary files, caches apps rebuild, crash reports and
// logs, the trash, and installers and old files left in Downloads.
//
// What's safe to delete is found in places each system keeps for it
// (see sources_*.go). Caches and temporary files are deleted for good,
// since the apps make them again; a download is moved to the trash, as
// it may be the only copy.
package cleanup

import (
	"path/filepath"
	"time"
)

// Kind is a sort of file that can be cleaned.
type Kind int

const (
	Temp         Kind = iota // temporary files
	Caches                   // caches apps rebuild
	BuildCaches              // developer tools' caches
	Logs                     // crash reports and logs
	Trash                    // the trash
	Installers               // installers in Downloads
	OldDownloads             // downloads untouched for a long while
	kinds
)

// Item is a file or folder to clean.
type Item struct {
	Kind     Kind
	Path     string
	Name     string // what to call it, when its path doesn't say
	Size     int64
	Files    int
	Modified time.Time // the newest file in it
	// Keep keeps the folder itself, emptying it: a place an app or the
	// system expects to find.
	Keep bool
}

// Group is the items of one kind, largest first.
type Group struct {
	Kind  Kind
	Items []Item
}

// Source is a place to look.
type Source struct {
	Kind Kind
	Dir  string
	// Name is what to call the place, such as the app whose cache it
	// is. Empty for a place whose contents are listed one by one.
	Name string
	// Each lists what's in Dir as items, rather than Dir as one.
	Each bool
}

// Progress says how a search is going.
type Progress struct {
	Kind  Kind
	Files int
	Bytes int64
}

// Result says what cleaning did.
type Result struct {
	Freed   int64 // deleted for good
	Trashed int64 // moved to the trash
	Files   int
	// Left counts files that couldn't be deleted, mostly because an app
	// has them open.
	Left int
}

// Space is how much a drive holds and has free.
type Space struct {
	Free, Total int64
}

// Label names a kind.
func (k Kind) Label() string {
	return [...]string{"Temporary files", "App caches", "Developer caches", "Crash reports and logs",
		"Trash", "Installers", "Old downloads"}[k]
}

// Safe says whether a kind can be cleaned without the user looking
// through it first: what's in it is made again or isn't needed.
func (k Kind) Safe() bool {
	switch k {
	case Temp, Caches, Logs:
		return true
	}
	return false
}

// ToTrash says whether a kind's files go to the trash, rather than
// being deleted, where there's a trash.
func (k Kind) ToTrash() bool { return k == Installers || k == OldDownloads }

// Kinds lists the kinds in the order they're shown.
func Kinds() []Kind {
	out := make([]Kind, kinds)
	for i := range out {
		out[i] = Kind(i)
	}
	return out
}

// Size is the space the group takes.
func (g Group) Size() int64 {
	var n int64
	for _, it := range g.Items {
		n += it.Size
	}
	return n
}

// Files counts the group's files.
func (g Group) Files() int {
	n := 0
	for _, it := range g.Items {
		n += it.Files
	}
	return n
}

// Title is what to call the item.
func (it Item) Title() string {
	if it.Name != "" {
		return it.Name
	}
	return filepath.Base(it.Path)
}

// Free says how full the drive holding path is.
func Free(path string) (Space, error) { return free(path) }
