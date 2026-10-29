package disk

import "io"

// Section is a window onto part of a device: a partition, or a volume
// found by a scan. Offsets passed to ReadAt are relative to its start.
type Section struct {
	r      io.ReaderAt
	Offset int64
	Length int64
}

// NewSection returns the n bytes of r starting at off.
func NewSection(r io.ReaderAt, off, n int64) *Section {
	return &Section{r: r, Offset: off, Length: n}
}

func (s *Section) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 || off >= s.Length {
		return 0, io.EOF
	}
	if max := s.Length - off; int64(len(p)) > max {
		n, err := s.r.ReadAt(p[:max], s.Offset+off)
		if err == nil {
			err = io.EOF
		}
		return n, err
	}
	return s.r.ReadAt(p, s.Offset+off)
}

// Size is the section's length.
func (s *Section) Size() int64 { return s.Length }
