package dupes

import (
	"context"
	"crypto/sha256"
	"io"
	"runtime"
	"sort"
	"sync"
)

// sample is how much of each end of a file is read before hashing it
// whole: most files of the same size differ there.
const sample = 16 << 10

// exact groups the files with the same contents, and tells which files
// are in a group.
func exact(ctx context.Context, files []File, progress func(Progress)) ([]Group, map[string]bool, error) {
	bySize := map[int64][]File{}
	for _, f := range files {
		bySize[f.Size] = append(bySize[f.Size], f)
	}
	var candidates [][]File
	total := 0
	for _, fs := range bySize {
		if len(fs) > 1 {
			candidates = append(candidates, fs)
			total += len(fs)
		}
	}
	// The largest first, where the most space is.
	sort.Slice(candidates, func(i, j int) bool { return candidates[i][0].Size > candidates[j][0].Size })

	var flat []File
	for _, c := range candidates {
		flat = append(flat, c...)
	}
	quick := make([][32]byte, len(flat))
	ok := make([]bool, len(flat))
	var t throttle
	report := func(phase string, k int) {
		if t.ok(k == len(flat)) {
			progress(Progress{Phase: PhaseHashing, Done: k / 2, Of: len(flat), Files: len(files)})
		}
	}
	parallel(ctx, len(flat), runtime.NumCPU(), func(i int) {
		quick[i], ok[i] = hashEnds(flat[i])
	}, func(k int) { report("ends", k) })
	if ctx.Err() != nil {
		return nil, nil, ctx.Err()
	}

	// Files whose ends match are hashed whole.
	type key struct {
		size int64
		sum  [32]byte
	}
	byQuick := map[key][]int{}
	for i, f := range flat {
		if ok[i] {
			byQuick[key{f.Size, quick[i]}] = append(byQuick[key{f.Size, quick[i]}], i)
		}
	}
	var again []int
	for _, is := range byQuick {
		if len(is) > 1 {
			again = append(again, is...)
		}
	}
	sort.Ints(again)
	full := map[int][32]byte{}
	var mu sync.Mutex
	parallel(ctx, len(again), min(runtime.NumCPU(), 4), func(j int) {
		i := again[j]
		if sum, ok := hashAll(ctx, flat[i]); ok {
			mu.Lock()
			full[i] = sum
			mu.Unlock()
		}
	}, func(k int) {
		if t.ok(k == len(again)) {
			progress(Progress{Phase: PhaseHashing, Done: len(flat)/2 + k*len(flat)/max(2*len(again), 1), Of: len(flat), Files: len(files)})
		}
	})
	if ctx.Err() != nil {
		return nil, nil, ctx.Err()
	}

	byFull := map[key][]File{}
	var order []key
	for _, i := range again {
		sum, ok := full[i]
		if !ok {
			continue
		}
		k := key{flat[i].Size, sum}
		if byFull[k] == nil {
			order = append(order, k)
		}
		byFull[k] = append(byFull[k], flat[i])
	}
	var groups []Group
	in := map[string]bool{}
	for _, k := range order {
		fs := byFull[k]
		if len(fs) < 2 {
			continue
		}
		groups = append(groups, Group{Kind: Exact, Files: fs})
		for _, f := range fs {
			in[f.Path] = true
		}
	}
	progress(Progress{Phase: PhaseHashing, Done: len(flat), Of: len(flat), Files: len(files), Groups: len(groups)})
	return groups, in, nil
}

// hashEnds hashes the start and the end of a file.
func hashEnds(f File) ([32]byte, bool) {
	r, err := open(f.Path)
	if err != nil {
		return [32]byte{}, false
	}
	defer r.Close()
	h := sha256.New()
	buf := make([]byte, sample)
	n, _ := io.ReadFull(r, buf)
	h.Write(buf[:n])
	if f.Size > 2*sample {
		n, _ = r.ReadAt(buf, f.Size-sample)
		h.Write(buf[:n])
	}
	var sum [32]byte
	h.Sum(sum[:0])
	return sum, true
}

// hashAll hashes a whole file.
func hashAll(ctx context.Context, f File) ([32]byte, bool) {
	r, err := open(f.Path)
	if err != nil {
		return [32]byte{}, false
	}
	defer r.Close()
	h := sha256.New()
	buf := make([]byte, 1<<20)
	for {
		if ctx.Err() != nil {
			return [32]byte{}, false
		}
		n, err := r.Read(buf)
		h.Write(buf[:n])
		if err == io.EOF {
			break
		}
		if err != nil {
			return [32]byte{}, false
		}
	}
	var sum [32]byte
	h.Sum(sum[:0])
	return sum, true
}
