package dupes

import (
	"context"
	"image"
	"math/bits"
	"runtime"
)

// fingerprint is a difference hash of a picture: it is shrunk to 9 by 8
// shades of gray, and each bit says whether a shade is brighter than the
// one to its right. Resizing, recompressing and small edits change few
// bits; a different picture changes about half.
func fingerprint(img image.Image) uint64 {
	var gray [8][9]float64
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	for y := range 8 {
		for x := range 9 {
			// Average a grid of samples over the cell.
			x0, x1 := b.Min.X+x*w/9, b.Min.X+(x+1)*w/9
			y0, y1 := b.Min.Y+y*h/8, b.Min.Y+(y+1)*h/8
			sx, sy := max((x1-x0)/6, 1), max((y1-y0)/6, 1)
			var sum float64
			n := 0
			for py := y0; py < max(y1, y0+1); py += sy {
				for px := x0; px < max(x1, x0+1); px += sx {
					r, g, bl, _ := img.At(px, py).RGBA()
					sum += 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(bl)
					n++
				}
			}
			gray[y][x] = sum / float64(n)
		}
	}
	var fp uint64
	for y := range 8 {
		for x := range 8 {
			fp <<= 1
			if gray[y][x] > gray[y][x+1] {
				fp |= 1
			}
		}
	}
	return fp
}

// readPhoto decodes a photo and takes its fingerprint.
func readPhoto(p string) photo {
	f, err := open(p)
	if err != nil {
		return photo{}
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil || cfg.Width*cfg.Height > maxPixels || cfg.Width < 16 || cfg.Height < 16 {
		return photo{}
	}
	if _, err := f.Seek(0, 0); err != nil {
		return photo{}
	}
	img, _, err := image.Decode(f)
	if err != nil {
		return photo{}
	}
	return photo{fp: fingerprint(img), ok: true, w: cfg.Width, h: cfg.Height}
}

// similarPhotos groups photos whose fingerprints differ in at most
// threshold bits, and whose shapes match.
func similarPhotos(ctx context.Context, files []File, threshold, found int, progress func(Progress)) ([]Group, error) {
	ps := make([]photo, len(files))
	var t throttle
	parallel(ctx, len(files), runtime.NumCPU(), func(i int) {
		ps[i] = readPhoto(files[i].Path)
	}, func(k int) {
		if t.ok(k == len(files)) {
			progress(Progress{Phase: PhasePhotos, Done: k, Of: len(files), Groups: found})
		}
	})
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	for i := range files {
		files[i].Width, files[i].Height = ps[i].w, ps[i].h
	}

	// Two fingerprints within threshold bits agree exactly on at least
	// one of threshold+1 slices, so only photos sharing a slice are
	// compared.
	bands := threshold + 1
	u := newUnion(len(files))
	dist := map[int]int{} // the largest distance joined into each set
	for band := range bands {
		lo, hi := band*64/bands, (band+1)*64/bands
		mask := uint64(1)<<(hi-lo) - 1
		buckets := map[uint64][]int{}
		for i, p := range ps {
			if p.ok {
				k := p.fp >> lo & mask
				buckets[k] = append(buckets[k], i)
			}
		}
		for _, is := range buckets {
			for a := 0; a < len(is); a++ {
				for b := a + 1; b < len(is); b++ {
					i, j := is[a], is[b]
					d := bits.OnesCount64(ps[i].fp ^ ps[j].fp)
					if d <= threshold && sameShape(ps[i].w, ps[i].h, ps[j].w, ps[j].h) {
						di, dj := dist[u.find(i)], dist[u.find(j)]
						u.join(i, j)
						dist[u.find(i)] = max(di, dj, d)
					}
				}
			}
		}
	}
	sets := map[int][]File{}
	var order []int
	for i := range files {
		if !ps[i].ok {
			continue
		}
		r := u.find(i)
		if sets[r] == nil {
			order = append(order, r)
		}
		sets[r] = append(sets[r], files[i])
	}
	var out []Group
	for _, r := range order {
		if len(sets[r]) > 1 {
			out = append(out, Group{Kind: SimilarPhoto, Files: sets[r], Distance: dist[r]})
		}
	}
	return out, nil
}

// sameShape tells whether two pictures have about the same proportions,
// either way up.
func sameShape(w1, h1, w2, h2 int) bool {
	ra := float64(w1) / float64(h1)
	rb := float64(w2) / float64(h2)
	near := func(x, y float64) bool { return x/y < 1.08 && y/x < 1.08 }
	return near(ra, rb) || near(ra, 1/rb)
}
