package dupes

import (
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

// photo is a photo's fingerprint.
type photo struct {
	fp   uint64
	ok   bool
	w, h int
}

// maxPixels skips pictures too large to decode without a lot of memory.
const maxPixels = 80_000_000

func (u *union) find(i int) int {
	for u.parent[i] != i {
		u.parent[i] = u.parent[u.parent[i]]
		i = u.parent[i]
	}
	return i
}

func (u *union) join(a, b int) { u.parent[u.find(a)] = u.find(b) }
