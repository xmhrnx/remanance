package dupes

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// list walks the roots. A file reached through two roots is listed once.
func list(ctx context.Context, roots []string, opts Options, progress func(Progress)) ([]File, error) {
	seen := map[string]bool{}
	var out []File
	last := time.Now()
	for _, root := range roots {
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err != nil {
				if d != nil && d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			name := d.Name()
			if p != root && !opts.Hidden && strings.HasPrefix(name, ".") {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if d.IsDir() || !d.Type().IsRegular() {
				return nil
			}
			info, err := d.Info()
			if err != nil || info.Size() == 0 || info.Size() < opts.MinSize {
				return nil
			}
			abs, _ := filepath.Abs(p)
			if seen[abs] {
				return nil
			}
			seen[abs] = true
			out = append(out, File{Path: p, Size: info.Size(), Modified: info.ModTime()})
			if time.Since(last) > 150*time.Millisecond {
				last = time.Now()
				progress(Progress{Phase: PhaseListing, Files: len(out)})
			}
			return nil
		})
		if err != nil && ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	progress(Progress{Phase: PhaseListing, Files: len(out)})
	return out, nil
}

// suggest marks the file to keep: the most pixels, then the largest,
// then the oldest, then the shortest path.
func suggest(g *Group) {
	best := 0
	for i, f := range g.Files {
		b := g.Files[best]
		switch {
		case f.Width*f.Height != b.Width*b.Height:
			if f.Width*f.Height > b.Width*b.Height {
				best = i
			}
		case f.Size != b.Size:
			if f.Size > b.Size {
				best = i
			}
		case !f.Modified.Equal(b.Modified):
			if f.Modified.Before(b.Modified) {
				best = i
			}
		case len(f.Path) < len(b.Path):
			best = i
		}
	}
	for i := range g.Files {
		g.Files[i].Keep = i == best
	}
	// The one to keep goes first.
	g.Files[0], g.Files[best] = g.Files[best], g.Files[0]
}

func ext(p string) string { return strings.TrimPrefix(strings.ToLower(filepath.Ext(p)), ".") }

func isPhoto(p string) bool {
	switch ext(p) {
	case "jpg", "jpeg", "png", "gif", "bmp", "tif", "tiff", "webp":
		return true
	}
	return false
}

func isVideo(p string) bool {
	switch ext(p) {
	case "mp4", "m4v", "mov", "3gp", "3g2":
		return true
	}
	return false
}

func open(p string) (*os.File, error) { return os.Open(p) }
