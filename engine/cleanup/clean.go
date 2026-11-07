package cleanup

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/xmhrnx/remanance/engine/trash"
)

// buildCaches are the folders developer tools keep their caches in,
// inside a system's cache folder. Clearing them is safe but makes the
// next build slow, so they're shown apart.
var buildCaches = map[string]bool{
	"go-build": true, "gopls": true, "pip": true, "pypoetry": true, "uv": true, "npm": true,
	"npm-cache": true, "yarn": true, "pnpm": true, "node-gyp": true, "ms-playwright": true,
	"cargo": true, "bazel": true, "gradle": true, "maven": true, "coursier": true,
	"jetbrains": true, "huggingface": true, "torch": true, "electron": true, "electron-builder": true,
	"homebrew": true, "cypress": true, "deno": true, "bun": true, "composer": true,
}

// installerExts are the kinds of file that install an app, which are
// rarely needed once it is.
var installerExts = map[string]bool{
	".exe": true, ".msi": true, ".msix": true, ".appx": true, ".dmg": true, ".pkg": true,
	".apk": true, ".xapk": true, ".deb": true, ".rpm": true, ".appimage": true, ".flatpakref": true,
}

func buildCache(name string) bool { return buildCaches[strings.ToLower(name)] }

func installer(name string) bool { return installerExts[strings.ToLower(filepath.Ext(name))] }

// Clean deletes the items, or moves them to the trash for the kinds
// that go there, and says how much space it freed. A file that can't be
// deleted is left and counted; only a failure to clean anything is an
// error.
func Clean(ctx context.Context, items []Item, progress func(done, of int)) (Result, error) {
	var r Result
	var first error
	for i, it := range items {
		if err := ctx.Err(); err != nil {
			return r, err
		}
		if progress != nil {
			progress(i, len(items))
		}
		switch {
		case it.Kind == Trash:
			if err := trash.Empty(); err != nil {
				first = errors.Join(first, err)
				continue
			}
			r.Freed += it.Size
			r.Files += it.Files
		case it.Kind.ToTrash():
			if err := trash.Remove(it.Path, true); err != nil {
				first = errors.Join(first, err)
				r.Left += it.Files
				continue
			}
			if trash.Supported {
				r.Trashed += it.Size
			} else {
				r.Freed += it.Size
			}
			r.Files += it.Files
		default:
			freed, files, left := remove(it.Path, it.Keep)
			r.Freed += freed
			r.Files += files
			r.Left += left
		}
	}
	if progress != nil {
		progress(len(items), len(items))
	}
	if r.Files == 0 && first != nil {
		return r, first
	}
	return r, nil
}

// remove deletes a file, or the files in a folder one by one so those in
// use are skipped rather than stopping it, then the folders left empty.
func remove(path string, keep bool) (freed int64, files, left int) {
	info, err := os.Lstat(path)
	if err != nil {
		return 0, 0, 0
	}
	if !info.IsDir() {
		if os.Remove(path) != nil {
			return 0, 0, 1
		}
		return info.Size(), 1, 0
	}
	var dirs []string
	filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			dirs = append(dirs, p)
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return nil
		}
		if os.Remove(p) != nil {
			left++
			return nil
		}
		freed += fi.Size()
		if fi.Mode().IsRegular() {
			files++
		}
		return nil
	})
	// Deepest first, so a folder's folders are gone before it.
	for i := len(dirs) - 1; i >= 0; i-- {
		if keep && dirs[i] == path {
			continue
		}
		os.Remove(dirs[i]) // fails, as it should, while something's left in it
	}
	return freed, files, left
}
