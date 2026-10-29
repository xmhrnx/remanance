package disk

import (
	"io"
	"sort"
	"sync"
)

// Range is a span of bytes on a device.
type Range struct {
	Offset, Length int64
}

// SafeReader keeps going over unreadable sectors: a failed read is retried
// sector by sector, sectors that still fail read as zeros and are recorded.
// Failing drives are a large share of recovery cases, and one bad sector
// must not stop a scan or a copy.
type SafeReader struct {
	r      io.ReaderAt
	size   int64
	sector int64

	mu  sync.Mutex
	bad []Range
}

// NewSafeReader wraps r, whose sectors are sectorSize bytes.
func NewSafeReader(r io.ReaderAt, size int64, sectorSize int) *SafeReader {
	if sectorSize <= 0 {
		sectorSize = 512
	}
	return &SafeReader{r: r, size: size, sector: int64(sectorSize)}
}

func (s *SafeReader) ReadAt(p []byte, off int64) (int, error) {
	if off >= s.size {
		return 0, io.EOF
	}
	want := len(p)
	eof := false
	if off+int64(want) > s.size {
		want = int(s.size - off)
		eof = true
	}
	n, err := s.r.ReadAt(p[:want], off)
	if err == nil || (err == io.EOF && n == want) {
		if eof {
			return n, io.EOF
		}
		return n, nil
	}
	// Retry what's missing one sector at a time.
	for pos := n - n%int(s.sector); pos < want; pos += int(s.sector) {
		end := pos + int(s.sector)
		if end > want {
			end = want
		}
		if m, e := s.r.ReadAt(p[pos:end], off+int64(pos)); e != nil && !(e == io.EOF && m == end-pos) {
			clear(p[pos+m : end])
			s.markBad(off+int64(pos), int64(end-pos))
		}
	}
	if eof {
		return want, io.EOF
	}
	return want, nil
}

func (s *SafeReader) markBad(off, n int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if l := len(s.bad); l > 0 && s.bad[l-1].Offset+s.bad[l-1].Length == off {
		s.bad[l-1].Length += n
		return
	}
	s.bad = append(s.bad, Range{off, n})
}

// BadRanges returns the unreadable spans found so far, sorted.
func (s *SafeReader) BadRanges() []Range {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]Range(nil), s.bad...)
	sort.Slice(out, func(i, j int) bool { return out[i].Offset < out[j].Offset })
	return out
}

// BadBytes is the total size of the unreadable spans.
func (s *SafeReader) BadBytes() int64 {
	var t int64
	for _, r := range s.BadRanges() {
		t += r.Length
	}
	return t
}
