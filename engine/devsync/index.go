package devsync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// Version is a version vector: how many changes each device made to a
// file, by the short form of its ID.
type Version map[string]uint64

// Compare tells how two versions relate.
type order int

const (
	equal      order = iota
	older            // v is older: other has every change v has, and more
	newer            // v is newer
	concurrent       // each has changes the other lacks
)

func (v Version) compare(other Version) order {
	less, more := false, false
	for k, n := range v {
		if n > other[k] {
			more = true
		} else if n < other[k] {
			less = true
		}
	}
	for k, n := range other {
		if _, ok := v[k]; !ok && n > 0 {
			less = true
		}
	}
	switch {
	case less && more:
		return concurrent
	case less:
		return older
	case more:
		return newer
	}
	return equal
}

// merge has every change of both.
func (v Version) merge(other Version) Version {
	out := Version{}
	for k, n := range v {
		out[k] = n
	}
	for k, n := range other {
		out[k] = max(out[k], n)
	}
	return out
}

func (v Version) bump(id string) Version {
	out := v.merge(nil)
	out[short(id)]++
	return out
}

// File is a file in a folder's index.
type File struct {
	Name    string  `json:"name"` // its path in the folder, with "/"
	Size    int64   `json:"size"`
	Mod     int64   `json:"mod"` // when it last changed, in Unix nanoseconds
	Hash    string  `json:"hash,omitempty"`
	Deleted bool    `json:"deleted,omitempty"`
	Version Version `json:"version"`
}

// index is a folder's files on this device.
type index struct {
	Files map[string]File `json:"files"`
}

func loadIndex(p string) *index {
	ix := &index{Files: map[string]File{}}
	if b, err := os.ReadFile(p); err == nil {
		json.Unmarshal(b, ix)
	}
	if ix.Files == nil {
		ix.Files = map[string]File{}
	}
	return ix
}

// ignored tells whether a name is DataDoc's own, or a system's file
// that shouldn't travel.
func ignored(name string) bool {
	base := path.Base(name)
	return strings.HasPrefix(base, ".datadoc-") || base == ".DS_Store" || base == "Thumbs.db" ||
		base == "desktop.ini" || strings.HasPrefix(base, "~$") || strings.HasPrefix(name, ".datadoc-")
}

// validName tells whether a name from another device stays inside the
// folder.
func validName(name string) bool {
	if name == "" || strings.HasPrefix(name, "/") || strings.Contains(name, "\\") || strings.ContainsRune(name, 0) {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return !ignored(name)
}

// scan compares the folder with its index: new and changed files are
// hashed and get a new version from this device, and files gone are
// marked deleted. It returns the files that changed.
func scan(ctx context.Context, root, self string, ix *index) ([]File, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New(root + " isn't a folder")
	}
	var changed []File
	seen := map[string]bool{}
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			if d != nil && d.IsDir() && p != root {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		name := filepath.ToSlash(rel)
		if p == root {
			return nil
		}
		if ignored(name) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return nil
		}
		seen[name] = true
		old, ok := ix.Files[name]
		if ok && !old.Deleted && old.Size == fi.Size() && old.Mod == fi.ModTime().UnixNano() {
			return nil
		}
		sum, err := hashFile(p)
		if err != nil {
			return nil // being written, or locked: next scan
		}
		f := File{Name: name, Size: fi.Size(), Mod: fi.ModTime().UnixNano(), Hash: sum}
		if ok && !old.Deleted && old.Hash == sum {
			// Only its date changed: same contents, same version.
			f.Version = old.Version
			ix.Files[name] = f
			return nil
		}
		f.Version = old.Version.bump(self)
		ix.Files[name] = f
		changed = append(changed, f)
		return nil
	})
	if err != nil {
		return nil, err
	}
	for name, f := range ix.Files {
		if !f.Deleted && !seen[name] {
			f.Deleted, f.Hash, f.Size = true, "", 0
			f.Mod = time.Now().UnixNano()
			f.Version = f.Version.bump(self)
			ix.Files[name] = f
			changed = append(changed, f)
		}
	}
	return changed, nil
}

func hashFile(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
