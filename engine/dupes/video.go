package dupes

import (
	"context"
	"encoding/binary"
	"runtime"
	"sort"
	"time"
)

// video is what a video's header says.
type video struct {
	dur  time.Duration
	w, h int
}

// readVideo reads an MP4 or QuickTime file's length from its movie
// header and its frame size from its largest track.
func readVideo(p string) (video, bool) {
	f, err := open(p)
	if err != nil {
		return video{}, false
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return video{}, false
	}
	var v video
	var walk func(off, end int64, depth int)
	walk = func(off, end int64, depth int) {
		h := make([]byte, 16)
		for off+8 <= end && depth < 6 {
			if _, err := f.ReadAt(h[:8], off); err != nil {
				return
			}
			size := int64(binary.BigEndian.Uint32(h))
			typ := string(h[4:8])
			head := int64(8)
			if size == 1 {
				if _, err := f.ReadAt(h[8:16], off+8); err != nil {
					return
				}
				size, head = int64(binary.BigEndian.Uint64(h[8:16])), 16
			} else if size == 0 {
				size = end - off
			}
			if size < head || off+size > end {
				return
			}
			switch typ {
			case "moov", "trak":
				walk(off+head, off+size, depth+1)
			case "mvhd":
				b := make([]byte, min(size-head, 32))
				if _, err := f.ReadAt(b, off+head); err == nil && len(b) >= 20 {
					var scale, dur uint64
					if b[0] == 1 && len(b) >= 32 {
						scale, dur = uint64(binary.BigEndian.Uint32(b[20:])), binary.BigEndian.Uint64(b[24:])
					} else {
						scale, dur = uint64(binary.BigEndian.Uint32(b[12:])), uint64(binary.BigEndian.Uint32(b[16:]))
					}
					if scale > 0 {
						v.dur = time.Duration(float64(dur) / float64(scale) * float64(time.Second))
					}
				}
			case "tkhd":
				// The frame size is the header's last 8 bytes, in 16.16
				// fixed point.
				if size-head >= 84 {
					b := make([]byte, 8)
					if _, err := f.ReadAt(b, off+size-8); err == nil {
						w, h := int(binary.BigEndian.Uint32(b)>>16), int(binary.BigEndian.Uint32(b[4:])>>16)
						if w*h > v.w*v.h {
							v.w, v.h = w, h
						}
					}
				}
			}
			off += size
		}
	}
	walk(0, info.Size(), 0)
	return v, v.dur >= time.Second && v.w > 0 && v.h > 0
}

// similarVideos groups videos of the same length, to a hundredth, and
// the same proportions: the same clip exported again or at another size.
func similarVideos(ctx context.Context, files []File, found int, progress func(Progress)) ([]Group, error) {
	vs := make([]video, len(files))
	ok := make([]bool, len(files))
	var t throttle
	parallel(ctx, len(files), runtime.NumCPU(), func(i int) {
		vs[i], ok[i] = readVideo(files[i].Path)
	}, func(k int) {
		if t.ok(k == len(files)) {
			progress(Progress{Phase: PhaseVideos, Done: k, Of: len(files), Groups: found})
		}
	})
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	var idx []int
	for i := range files {
		if ok[i] {
			files[i].Width, files[i].Height, files[i].Duration = vs[i].w, vs[i].h, vs[i].dur
			idx = append(idx, i)
		}
	}
	sort.Slice(idx, func(a, b int) bool { return vs[idx[a]].dur < vs[idx[b]].dur })
	u := newUnion(len(files))
	for a := 0; a < len(idx); a++ {
		for b := a + 1; b < len(idx); b++ {
			i, j := idx[a], idx[b]
			// Within a hundredth of the length, and at least 50ms.
			if float64(vs[j].dur-vs[i].dur) > max(float64(vs[i].dur)/100, float64(50*time.Millisecond)) {
				break
			}
			if sameShape(vs[i].w, vs[i].h, vs[j].w, vs[j].h) {
				u.join(i, j)
			}
		}
	}
	sets := map[int][]File{}
	var order []int
	for _, i := range idx {
		r := u.find(i)
		if sets[r] == nil {
			order = append(order, r)
		}
		sets[r] = append(sets[r], files[i])
	}
	var out []Group
	for _, r := range order {
		if len(sets[r]) > 1 {
			out = append(out, Group{Kind: SimilarVideo, Files: sets[r]})
		}
	}
	return out, nil
}
