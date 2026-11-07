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
	"sort"
	"time"
)

const (
	Exact        Kind = iota // the same bytes
	SimilarPhoto             // the same picture
	SimilarVideo             // the same length and shape
)

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

const (
	PhaseListing Phase = iota
	PhaseHashing
	PhasePhotos
	PhaseVideos
)

// Progress says how a search is going.
type Progress struct {
	Phase      Phase
	Done, Of   int // files in this phase
	Files      int // files listed
	Groups     int // groups found so far
	Bytes      int64
	BytesShown int64 // read so far while hashing
}

// Label says how a group's files are alike.
func (k Kind) Label() string {
	return [...]string{"Exact copies", "Similar photos", "Similar videos"}[k]
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

// Label names a phase.
func (p Phase) Label() string {
	return [...]string{"Listing files", "Comparing contents", "Comparing photos", "Comparing videos"}[p]
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
