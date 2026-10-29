// Package imaging copies a whole device to an image file. Recovering from
// an image instead of a failing drive reads the drive only once.
package imaging

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mehran9675/remanence/engine/disk"
)

// Progress reports an imaging run.
type Progress struct {
	Done, Total int64
	BadBytes    int64
	Speed       float64
}

// Result sums up an imaging run.
type Result struct {
	Path     string
	Size     int64
	Bad      []disk.Range
	Duration time.Duration
}

// Create writes dev to path. Unreadable sectors are written as zeros and
// listed in path + ".bad", one "offset length" pair per line.
func Create(ctx context.Context, dev disk.Device, path string, progress func(Progress)) (*Result, error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	safe := disk.NewSafeReader(dev, dev.Size(), dev.SectorSize())
	const chunk = 8 << 20
	buf := make([]byte, chunk)
	start := time.Now()
	total := dev.Size()
	last := time.Time{}
	for off := int64(0); off < total; off += chunk {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		n := min(int64(chunk), total-off)
		if _, err := safe.ReadAt(buf[:n], off); err != nil && err != io.EOF {
			return nil, err
		}
		if _, err := f.Write(buf[:n]); err != nil {
			return nil, fmt.Errorf("writing the image: %w", err)
		}
		if progress != nil && time.Since(last) > 200*time.Millisecond {
			last = time.Now()
			progress(Progress{Done: off + n, Total: total, BadBytes: safe.BadBytes(), Speed: float64(off+n) / time.Since(start).Seconds()})
		}
	}
	if err := f.Sync(); err != nil {
		return nil, err
	}
	res := &Result{Path: path, Size: total, Bad: safe.BadRanges(), Duration: time.Since(start)}
	if len(res.Bad) > 0 {
		if bf, err := os.Create(path + ".bad"); err == nil {
			for _, r := range res.Bad {
				fmt.Fprintf(bf, "%d %d\n", r.Offset, r.Length)
			}
			bf.Close()
		}
	}
	if progress != nil {
		progress(Progress{Done: total, Total: total, BadBytes: safe.BadBytes()})
	}
	return res, nil
}
