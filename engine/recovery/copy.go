package recovery

import (
	"context"
	"io"

	"github.com/xmhrnx/remanance/engine/fsys"
)

func copyEntry(ctx context.Context, dev io.ReaderAt, e *fsys.Entry, sink Sink, rel string, buf []byte, step func(int64)) (int64, error) {
	r, err := fsys.Open(dev, e)
	if err != nil {
		return 0, err
	}
	w, err := sink.Create(rel)
	if err != nil {
		return 0, err
	}
	var done int64
	for done < e.Size {
		if ctx.Err() != nil {
			w.Close()
			return done, ctx.Err()
		}
		n := min(int64(len(buf)), e.Size-done)
		m, rerr := r.ReadAt(buf[:n], done)
		if m > 0 {
			if _, werr := w.Write(buf[:m]); werr != nil {
				w.Close()
				return done, werr
			}
			done += int64(m)
			step(int64(m))
		}
		if rerr != nil && rerr != io.EOF {
			w.Close()
			return done, rerr
		}
		if m == 0 {
			break
		}
	}
	return done, w.Close()
}
