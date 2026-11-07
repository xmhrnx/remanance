package disk

import (
	"io"
	"sync"
)

// Range is a span of bytes on a device.
type Range struct {
	Offset, Length int64
}

// How SafeReader narrows down a failed read: in pieces of retryPiece
// bytes, then sector by sector inside a piece that fails, giving up on
// the rest of the piece after giveUpAfter bad sectors in a row. Reading
// every sector of a dying area one by one takes hours and is what makes
// a weak USB stick reset itself off the bus.
const (
	retryPiece  = 64 << 10
	giveUpAfter = 8
)

// SafeReader keeps going over unreadable sectors: a failed read is
// retried in smaller pieces, sectors that still fail read as zeros and
// are recorded. Failing drives are a large share of recovery cases, and
// one bad sector must not stop a scan or a copy.
//
// A drive that disconnects is different: every read after it fails, so
// SafeReader returns an error matching ErrGone rather than zeros.
type SafeReader struct {
	r      io.ReaderAt
	size   int64
	sector int64

	mu   sync.Mutex
	bad  []Range // sorted, merged
	gone error
}

// NewSafeReader wraps r, whose sectors are sectorSize bytes.
func NewSafeReader(r io.ReaderAt, size int64, sectorSize int) *SafeReader {
	if sectorSize <= 0 {
		sectorSize = 512
	}
	return &SafeReader{r: r, size: size, sector: int64(sectorSize)}
}

func (s *SafeReader) ReadAt(p []byte, off int64) (int, error) {
	if err := s.Gone(); err != nil {
		return 0, err
	}
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
	if IsGone(err) {
		return n, s.setGone(err)
	}
	// Retry what's missing, a piece at a time.
	from := n - n%int(s.sector)
	for pos := from; pos < want; pos += retryPiece {
		end := min(pos+retryPiece, want)
		if err := s.retry(p[pos:end], off+int64(pos), pos == from && from == 0 && end == want); err != nil {
			return pos, err
		}
	}
	if eof {
		return want, io.EOF
	}
	return want, nil
}

// retry reads one piece, sector by sector if it fails as a whole. tried
// says the piece is the read that already failed, so it goes straight to
// sectors.
func (s *SafeReader) retry(p []byte, off int64, tried bool) error {
	if !tried {
		m, err := s.r.ReadAt(p, off)
		if err == nil || (err == io.EOF && m == len(p)) {
			return nil
		}
		if IsGone(err) {
			return s.setGone(err)
		}
	}
	sec := int(s.sector)
	failed := 0
	for pos := 0; pos < len(p); pos += sec {
		end := min(pos+sec, len(p))
		if failed >= giveUpAfter {
			clear(p[pos:])
			s.markBad(off+int64(pos), int64(len(p)-pos))
			return nil
		}
		m, err := s.r.ReadAt(p[pos:end], off+int64(pos))
		switch {
		case err == nil || (err == io.EOF && m == end-pos):
			failed = 0
		case IsGone(err):
			return s.setGone(err)
		default:
			clear(p[pos+m : end])
			s.markBad(off+int64(pos), int64(end-pos))
			failed++
		}
	}
	return nil
}

func (s *SafeReader) setGone(err error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.gone == nil {
		s.gone = gone(err)
	}
	return s.gone
}

// Gone returns the error that ended reading, matching ErrGone, once the
// drive has disconnected; nil until then.
func (s *SafeReader) Gone() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.gone
}
