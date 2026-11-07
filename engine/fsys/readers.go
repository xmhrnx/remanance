package fsys

import (
	"io"
	"sync"
)

// inlineReader reads an entry's data kept inside its metadata.
type inlineReader struct {
	data []byte
	size int64
}

// runReader reads an entry's data from the device, run by run.
type runReader struct {
	dev  io.ReaderAt
	runs []Run
	size int64
}

// unitReader reads an entry's data through a codec, a unit at a time.
type unitReader struct {
	dev   io.ReaderAt
	e     *Entry
	codec Codec

	mu      sync.Mutex
	unitN   int64
	unitBuf []byte
}

func (r *inlineReader) ReadAt(p []byte, off int64) (int, error) {
	if off >= r.size {
		return 0, io.EOF
	}
	n := 0
	for n < len(p) && off+int64(n) < r.size {
		i := off + int64(n)
		if i < int64(len(r.data)) {
			n += copy(p[n:], r.data[i:min(int64(len(r.data)), r.size)])
		} else {
			p[n] = 0
			n++
		}
	}
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

func (r *runReader) ReadAt(p []byte, off int64) (int, error) {
	if off >= r.size {
		return 0, io.EOF
	}
	want := int64(len(p))
	if off+want > r.size {
		want = r.size - off
	}
	done := int64(0)
	pos := int64(0) // logical start of the current run
	for _, run := range r.runs {
		if done >= want {
			break
		}
		end := pos + run.Length
		at := off + done
		if at >= end {
			pos = end
			continue
		}
		in := at - pos
		n := min(run.Length-in, want-done)
		chunk := p[done : done+n]
		if run.Offset < 0 {
			clear(chunk)
		} else if m, err := r.dev.ReadAt(chunk, run.Offset+in); err != nil && !(err == io.EOF && int64(m) == n) {
			return int(done) + m, err
		}
		done += n
		pos = end
	}
	if done < want {
		// Runs shorter than the size: the rest was never written.
		clear(p[done:want])
		done = want
	}
	if done < int64(len(p)) {
		return int(done), io.EOF
	}
	return int(done), nil
}

func (r *unitReader) unit(n int64) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.unitBuf != nil && r.unitN == n {
		return r.unitBuf, nil
	}
	b, err := r.codec(r.dev, r.e, n)
	if err != nil {
		return nil, err
	}
	r.unitN, r.unitBuf = n, b
	return b, nil
}

func (r *unitReader) ReadAt(p []byte, off int64) (int, error) {
	size := r.e.Size
	if off >= size {
		return 0, io.EOF
	}
	done := 0
	for done < len(p) && off+int64(done) < size {
		at := off + int64(done)
		u, err := r.unit(at / r.e.Unit)
		if err != nil {
			return done, err
		}
		in := at % r.e.Unit
		if in >= int64(len(u)) {
			return done, io.ErrUnexpectedEOF
		}
		n := copy(p[done:], u[in:min(int64(len(u)), in+size-at)])
		done += n
	}
	if done < len(p) {
		return done, io.EOF
	}
	return done, nil
}
