// Package fsys is the file-system-neutral model the recovery engine works
// with: a tree of entries, existing and deleted, each knowing where its
// bytes are on the device.
package fsys

import (
	"path"
	"strings"
	"time"
)

// State tells how an entry was found.
type State uint8

const (
	Existing State = iota // listed by the file system
	Deleted               // marked deleted, metadata still present
	Orphan                // its folder is gone; found without a parent
	Carved                // found by its content, without any metadata
)

func (s State) String() string {
	switch s {
	case Existing:
		return "Existing"
	case Deleted:
		return "Deleted"
	case Orphan:
		return "Orphaned"
	case Carved:
		return "Found by signature"
	}
	return ""
}

// Run is a contiguous stretch of an entry's data on the device. Offset is
// absolute on the device; -1 marks a sparse run that reads as zeros.
type Run struct {
	Offset int64
	Length int64
}

// Entry is a file or folder.
type Entry struct {
	ID       uint64 // MFT record, inode, directory entry offset: unique per volume
	Name     string
	IsDir    bool
	Size     int64
	Created  time.Time
	Modified time.Time
	Accessed time.Time
	State    State

	// Health estimates, 0 to 100, how much of the data is still the
	// file's own: the share of its clusters nothing else has claimed
	// since it was deleted. Existing files are 100.
	Health uint8
	// Note explains a low health or a limitation (encrypted, fragmented).
	Note string

	Runs   []Run  // where the data is
	Inline []byte // data kept inside the metadata (small NTFS files)
	Codec  string // decoder needed for Runs ("lznt1"), "" for plain data
	Unit   int64  // codec block size in bytes (an NTFS compression unit)

	Parent   *Entry
	Children []*Entry
}

// Ext is the lower-case extension without the dot.
func (e *Entry) Ext() string {
	x := path.Ext(e.Name)
	if x == "" {
		return ""
	}
	return strings.ToLower(x[1:])
}

// Path is the entry's path from the volume root, with "/" separators.
func (e *Entry) Path() string {
	var parts []string
	for n := e; n != nil && n.Parent != nil; n = n.Parent {
		parts = append(parts, n.Name)
	}
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return "/" + strings.Join(parts, "/")
}

// Add makes child a child of e.
func (e *Entry) Add(child *Entry) {
	child.Parent = e
	e.Children = append(e.Children, child)
}

// Recoverable is true when there is data to copy (or nothing to copy, for
// an empty file).
func (e *Entry) Recoverable() bool {
	return e.IsDir || e.Size == 0 || len(e.Inline) > 0 || len(e.Runs) > 0
}

// Walk calls fn for e and every entry under it, depth first; fn returning
// false skips that entry's children.
func (e *Entry) Walk(fn func(*Entry) bool) {
	if !fn(e) {
		return
	}
	for _, c := range e.Children {
		c.Walk(fn)
	}
}

// HasDeleted reports whether e or anything under it is deleted, orphaned
// or carved.
func (e *Entry) HasDeleted() bool {
	found := false
	e.Walk(func(n *Entry) bool {
		if found {
			return false
		}
		if n.State != Existing {
			found = true
		}
		return !found
	})
	return found
}
