package carve

import (
	"encoding/binary"
	"io"
)

// stream reads a candidate sequentially in large chunks; most parsers walk
// a file front to back.
type stream struct {
	r     io.ReaderAt
	limit int64
	buf   []byte
	start int64 // offset of buf[0]
	pos   int64
}

func newStream(r io.ReaderAt, limit int64) *stream {
	return &stream{r: r, limit: limit, start: -1}
}

const streamChunk = 256 << 10

func (s *stream) fill(at int64) bool {
	if at >= s.limit {
		return false
	}
	if s.start >= 0 && at >= s.start && at < s.start+int64(len(s.buf)) {
		return true
	}
	n := min(int64(streamChunk), s.limit-at)
	if cap(s.buf) < int(n) {
		s.buf = make([]byte, n)
	}
	s.buf = s.buf[:n]
	m, _ := s.r.ReadAt(s.buf, at)
	s.buf = s.buf[:m]
	s.start = at
	return m > 0
}

// byteAt returns the byte at offset at, ok false past the end.
func (s *stream) byteAt(at int64) (byte, bool) {
	if !s.fill(at) {
		return 0, false
	}
	return s.buf[at-s.start], true
}

// next reads the byte at the current position and advances.
func (s *stream) next() (byte, bool) {
	b, ok := s.byteAt(s.pos)
	if ok {
		s.pos++
	}
	return b, ok
}

// at reads n bytes at off; nil when they're past the limit.
func at(r io.ReaderAt, off int64, n int, limit int64) []byte {
	if off < 0 || off+int64(n) > limit {
		return nil
	}
	b := make([]byte, n)
	if m, _ := r.ReadAt(b, off); m < n {
		return nil
	}
	return b
}

func le16(b []byte) uint16 { return binary.LittleEndian.Uint16(b) }
func le32(b []byte) uint32 { return binary.LittleEndian.Uint32(b) }
func le64(b []byte) uint64 { return binary.LittleEndian.Uint64(b) }
func be16(b []byte) uint16 { return binary.BigEndian.Uint16(b) }
func be32(b []byte) uint32 { return binary.BigEndian.Uint32(b) }
func be64(b []byte) uint64 { return binary.BigEndian.Uint64(b) }

// MB and GB are size limits for signatures.
const (
	KB = 1 << 10
	MB = 1 << 20
	GB = 1 << 30
)
