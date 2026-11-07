package fsys

import (
	"context"
	"fmt"
	"io"
	"sort"
	"sync"
)

// Volume is a parsed file system.
type Volume struct {
	FS          string
	Label       string
	Offset      int64 // where it starts on the device
	Size        int64
	ClusterSize int64
	Root        *Entry
	// Lost holds entries whose folder couldn't be found, in reconstructed
	// folders where their parent's identity is known.
	Lost *Entry
	// Warnings are problems met while parsing that didn't stop it.
	Warnings []string
}

// Stats counts what the volume holds.
type Stats struct {
	Files, Folders, Deleted int
	DeletedBytes            int64
}

// Progress receives how far a parser is, from 0 to 1.
type Progress func(done float64)

// Parser reads the file system in r, which is the volume itself (offset
// 0 is its first byte). devOffset is where the volume starts on the
// device; parsers add it to every Run so recovery reads the device
// directly.
type Parser func(ctx context.Context, r io.ReaderAt, devOffset, size int64, progress Progress) (*Volume, error)

var (
	parsersMu sync.RWMutex
	parsers   = map[string]Parser{}
)

// NewVolume returns a volume with an empty root and lost-files folder.
func NewVolume(fs string, offset, size int64) *Volume {
	v := &Volume{FS: fs, Offset: offset, Size: size}
	v.Root = &Entry{Name: "", IsDir: true, Health: 100}
	v.Lost = &Entry{Name: "Lost and found", IsDir: true, State: Orphan, Health: 100}
	return v
}

// Warn records a non-fatal problem.
func (v *Volume) Warn(format string, args ...any) {
	v.Warnings = append(v.Warnings, fmt.Sprintf(format, args...))
}

// Stats walks the tree and counts files, folders and deleted files.
func (v *Volume) Stats() Stats {
	var s Stats
	count := func(e *Entry) bool {
		if e.IsDir {
			s.Folders++
			return true
		}
		s.Files++
		if e.State != Existing {
			s.Deleted++
			s.DeletedBytes += e.Size
		}
		return true
	}
	v.Root.Walk(count)
	v.Lost.Walk(count)
	return s
}

// Sort orders every folder: folders first, then by name.
func (v *Volume) Sort() {
	sortChildren := func(e *Entry) bool {
		sort.SliceStable(e.Children, func(i, j int) bool {
			a, b := e.Children[i], e.Children[j]
			if a.IsDir != b.IsDir {
				return a.IsDir
			}
			return a.Name < b.Name
		})
		return true
	}
	v.Root.Walk(sortChildren)
	v.Lost.Walk(sortChildren)
}

// RegisterParser makes a parser available for the file systems named.
func RegisterParser(p Parser, names ...string) {
	parsersMu.Lock()
	defer parsersMu.Unlock()
	for _, n := range names {
		parsers[n] = p
	}
}

// ParserFor returns the parser for a file system name, nil if none.
func ParserFor(fs string) Parser {
	parsersMu.RLock()
	defer parsersMu.RUnlock()
	return parsers[fs]
}
