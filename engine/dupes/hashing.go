package dupes

import (
	"context"
	"crypto/sha256"
	"io"
)

// sample is how much of each end of a file is read before hashing it
// whole: most files of the same size differ there.
const sample = 16 << 10

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
