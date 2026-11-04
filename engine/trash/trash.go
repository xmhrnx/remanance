// Package trash moves files to the system's trash, where the user can
// put them back: the Recycle Bin on Windows, the Trash on macOS and the
// freedesktop.org trash on Linux. Phones have none for apps to use.
package trash

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// ErrNoTrash means the file can't go to the trash: the system has none,
// or the file is on a drive the trash doesn't cover.
var ErrNoTrash = errors.New("this file can't go to the trash")

// Supported is true where files can go to the trash.
const Supported = supported

// Move moves the file or folder at path to the trash.
func Move(path string) error {
	if _, err := os.Lstat(path); err != nil {
		return err
	}
	return move(path)
}

// Remove moves path to the trash where there is one, or deletes it.
func Remove(path string, toTrash bool) error {
	if toTrash && Supported {
		return Move(path)
	}
	return os.RemoveAll(path)
}

// Usage is what the trash holds.
type Usage struct {
	Bytes int64
	Files int
}

// Size is what the user's trash holds; zero where there's no trash.
func Size() (Usage, error) { return usage() }

// Empty deletes everything in the user's trash for good.
func Empty() error { return empty() }

// dirUsage adds up the files under dir.
func dirUsage(dir string) (Usage, error) {
	var u Usage
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == dir {
				return err
			}
			return nil
		}
		if d.Type().IsRegular() {
			if info, err := d.Info(); err == nil {
				u.Bytes += info.Size()
				u.Files++
			}
		}
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		err = nil
	}
	return u, err
}

// clearDir deletes what's in dir, keeping dir.
func clearDir(dir string) error {
	es, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var first error
	for _, e := range es {
		if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil && first == nil {
			first = err
		}
	}
	return first
}
