package disk

import "io"

// alignedReader turns arbitrary reads into reads of whole sectors at
// sector-aligned offsets, which raw devices on Windows and macOS require.
type alignedReader struct {
	r      io.ReaderAt
	sector int64
	size   int64
}

// Aligned wraps r so callers may read at any offset and length.
func Aligned(r io.ReaderAt, sectorSize int, size int64) io.ReaderAt {
	if sectorSize <= 0 {
		sectorSize = 512
	}
	return &alignedReader{r: r, sector: int64(sectorSize), size: size}
}

func (a *alignedReader) ReadAt(p []byte, off int64) (int, error) {
	if off >= a.size {
		return 0, io.EOF
	}
	want := int64(len(p))
	eof := false
	if off+want > a.size {
		want = a.size - off
		eof = true
	}
	start := off - off%a.sector
	end := off + want
	if rem := end % a.sector; rem != 0 {
		end += a.sector - rem
	}
	if start == off && end == off+want {
		n, err := a.r.ReadAt(p[:want], off)
		if err == nil && eof {
			err = io.EOF
		}
		return n, err
	}
	buf := make([]byte, end-start)
	n, err := a.r.ReadAt(buf, start)
	skip := int(off - start)
	if n <= skip {
		if err == nil {
			err = io.ErrUnexpectedEOF
		}
		return 0, err
	}
	c := copy(p[:want], buf[skip:n])
	if int64(c) < want {
		if err == nil {
			err = io.ErrUnexpectedEOF
		}
		return c, err
	}
	if eof {
		return c, io.EOF
	}
	return c, nil
}
