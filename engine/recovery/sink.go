// Package recovery copies found files out of a device to a destination.
package recovery

import (
	"io"
	"os"
	"path/filepath"
	"time"
)

// Sink is where recovered files are written: a folder on desktops, the
// Downloads collection on Android, the app's documents on iOS.
type Sink interface {
	// Create makes the file at rel, a slash-separated path. Parent
	// folders are created as needed.
	Create(rel string) (io.WriteCloser, error)
	// Finish sets the file's times once written, where supported.
	Finish(rel string, modified, accessed time.Time)
	// Exists reports whether rel is taken.
	Exists(rel string) bool
	// Location describes the destination to people.
	Location() string
}

// DirSink writes into a folder.
type DirSink struct{ Dir string }

func (d DirSink) path(rel string) string { return filepath.Join(d.Dir, filepath.FromSlash(rel)) }

func (d DirSink) Create(rel string) (io.WriteCloser, error) {
	p := d.path(rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return nil, err
	}
	return os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
}

func (d DirSink) Finish(rel string, modified, accessed time.Time) {
	if modified.IsZero() {
		return
	}
	if accessed.IsZero() {
		accessed = modified
	}
	os.Chtimes(d.path(rel), accessed, modified)
}

func (d DirSink) Exists(rel string) bool {
	_, err := os.Lstat(d.path(rel))
	return err == nil
}

func (d DirSink) Location() string { return d.Dir }
