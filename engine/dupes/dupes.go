// Package dupes finds files that are the same, and photos and videos
// that look the same: copies with other names, photos saved again at
// another size or quality, and the same clip exported twice.
//
// Exact copies are found by content: files of the same size are hashed,
// first a little from each end, then whole. Similar photos are compared
// by a fingerprint of their picture (see fingerprint.go), which stays
// close through resizing, recompression and small edits. Similar videos
// are matched by length and shape, read from the file's header.
package dupes

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Kind is how the files in a group are alike.
type Kind int

const (
	Exact        Kind = iota // the same bytes
	SimilarPhoto             // the same picture
	SimilarVideo             // the same length and shape
)

// Label says how a group's files are alike.
func (k Kind) Label() string {
	return [...]string{"Exact copies", "Similar photos", "Similar videos"}[k]
}

// File is one file in a group.
type File struct {
	Path          string
	Size          int64
	Modified      time.Time
	Width, Height int           // of a photo or video, when known
	Duration      time.Duration // of a video
	// Keep marks the file the group suggests keeping: the largest
	// picture, then the oldest copy.
	Keep bool
}

// Group is files that are alike.
type Group struct {
	Kind  Kind
	Files []File
	// Distance is how far apart the most different photos' fingerprints
	// are, out of 64: 0 is the same picture.
	Distance int
}

// Wasted is the space freed by keeping one file of the group.
func (g Group) Wasted() int64 {
	var sum, most int64
	for _, f := range g.Files {
		sum += f.Size
		most = max(most, f.Size)
	}
	return sum - most
}

// Options says what to look for.
type Options struct {
	// Similar also looks for photos and videos that look the same.
	Similar bool
	// Threshold is how many of a fingerprint's 64 bits may differ for
	// photos to count as similar: 0 to 12, 6 by default.
	Threshold int
	// MinSize skips files smaller than this.
	MinSize int64
	// Hidden includes hidden files and folders, whose names start with
	// a dot.
	Hidden bool
}

// Phase is a step of the search.
type Phase int

const (
	PhaseListing Phase = iota
	PhaseHashing
	PhasePhotos
	PhaseVideos
)

// Label names a phase.
func (p Phase) Label() string {
	return [...]string{"Listing files", "Comparing contents", "Comparing photos", "Comparing videos"}[p]
}

// Progress says how a search is going.
type Progress struct {
	Phase      Phase
	Done, Of   int // files in this phase
	Files      int // files listed
	Groups     int // groups found so far
	Bytes      int64
	BytesShown int64 // read so far while hashing
}

// Find looks through roots for files that are alike, and returns the
// groups, the most space wasted first.
func Find(ctx context.Context, roots []string, opts Options, progress func(Progress)) ([]Group, error) {
	if opts.Threshold == 0 {
		opts.Threshold = 6
	}
	opts.Threshold = min(max(opts.Threshold, 0), 12)
	if progress == nil {
		progress = func(Progress) {}
	}
	files, err := list(ctx, roots, opts, progress)
	if err != nil {
		return nil, err
	}
	groups, inExact, err := exact(ctx, files, progress)
	if err != nil {
		return nil, err
	}
	if opts.Similar {
		var photos, videos []File
		for _, f := range files {
			if inExact[f.Path] {
				continue
			}
			switch {
			case isPhoto(f.Path):
				photos = append(photos, f)
			case isVideo(f.Path):
				videos = append(videos, f)
			}
		}
		// One of each set of exact copies stands for it.
		for _, g := range groups {
			f := g.Files[0]
			switch {
			case isPhoto(f.Path):
				photos = append(photos, f)
			case isVideo(f.Path):
				videos = append(videos, f)
			}
		}
		sim, err := similarPhotos(ctx, photos, opts.Threshold, len(groups), progress)
		if err != nil {
			return nil, err
		}
		groups = append(groups, sim...)
		vid, err := similarVideos(ctx, videos, len(groups), progress)
		if err != nil {
			return nil, err
		}
		groups = append(groups, vid...)
	}
	for i := range groups {
		suggest(&groups[i])
	}
	sort.SliceStable(groups, func(i, j int) bool { return groups[i].Wasted() > groups[j].Wasted() })
	return groups, nil
}

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

// parallel runs fn on every index below n on all cores, and reports
// each one done.
func parallel(ctx context.Context, n, workers int, fn func(i int), done func(k int)) {
	var next, count atomic.Int64
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				i := int(next.Add(1) - 1)
				if i >= n {
					return
				}
				fn(i)
				done(int(count.Add(1)))
			}
		}()
	}
	wg.Wait()
}

// throttle lets a report through at most every 150ms, and the last one.
type throttle struct {
	mu   sync.Mutex
	last time.Time
}

func (t *throttle) ok(final bool) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !final && time.Since(t.last) < 150*time.Millisecond {
		return false
	}
	t.last = time.Now()
	return true
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
