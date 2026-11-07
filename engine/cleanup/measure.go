package cleanup

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
)

// measure adds up a file, or the files in a folder, and finds the
// newest change in it.
func measure(ctx context.Context, path string) (Item, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return Item{}, err
	}
	it := Item{Path: path, Modified: info.ModTime()}
	if !info.IsDir() {
		if info.Mode().IsRegular() {
			it.Size, it.Files = info.Size(), 1
		}
		return it, nil
	}
	err = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable: left out
		}
		if it.Files%512 == 0 && ctx.Err() != nil {
			return ctx.Err()
		}
		if !d.Type().IsRegular() {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return nil
		}
		it.Size += fi.Size()
		it.Files++
		if fi.ModTime().After(it.Modified) {
			it.Modified = fi.ModTime()
		}
		return nil
	})
	return it, err
}

// places makes a source of each folder matching a pattern, such as a
// browser's cache in each of its profiles.
func places(kind Kind, name string, patterns ...string) []Source {
	var out []Source
	for _, pat := range patterns {
		dirs, _ := filepath.Glob(pat)
		for _, d := range dirs {
			out = append(out, Source{Kind: kind, Dir: d, Name: name})
		}
	}
	return out
}
