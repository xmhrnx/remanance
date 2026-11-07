package scan

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xmhrnx/remanance/engine/disk"
	"github.com/xmhrnx/remanance/engine/fsys"
)

// memDrive is a drive in memory.
type memDrive struct{ data []byte }

// vanishing is a drive that drops off the bus after reading budget
// bytes, as a failing USB stick does: every read after that fails.
type vanishing struct {
	disk.Device
	budget atomic.Int64
}

func (m *memDrive) ReadAt(p []byte, off int64) (int, error) {
	return bytes.NewReader(m.data).ReadAt(p, off)
}
func (m *memDrive) Info() disk.Info { return disk.Info{Name: "test"} }
func (m *memDrive) Size() int64     { return int64(len(m.data)) }
func (m *memDrive) SectorSize() int { return 512 }
func (m *memDrive) Close() error    { return nil }

func (v *vanishing) ReadAt(p []byte, off int64) (int, error) {
	if v.budget.Add(-int64(len(p))) < 0 {
		return 0, fmt.Errorf("read: %w", disk.ErrGone)
	}
	return v.Device.ReadAt(p, off)
}

// pictures is a 64 MB drive with no file system: a PNG every megabyte,
// only carving finds them.
func pictures(t *testing.T) *memDrive {
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for i := range img.Pix {
		img.Pix[i] = byte(i * 7)
	}
	data := make([]byte, 64<<20)
	for mb := 1; mb < 64; mb++ {
		img.Set(0, 0, color.RGBA{byte(mb), 0, 0, 255})
		var b bytes.Buffer
		if err := png.Encode(&b, img); err != nil {
			t.Fatal(err)
		}
		copy(data[mb<<20:], b.Bytes())
	}
	return &memDrive{data: data}
}

func carvedFiles(res *Result) int {
	n := 0
	if res.Carved != nil {
		res.Carved.Root.Walk(func(e *fsys.Entry) bool {
			if !e.IsDir {
				n++
			}
			return true
		})
	}
	return n
}

func TestDriveDisconnectsDuringDeepScan(t *testing.T) {
	drive := pictures(t)
	whole, err := Run(context.Background(), drive, Options{Mode: Deep}, nil)
	if err != nil {
		t.Fatal(err)
	}
	all := carvedFiles(whole)
	if all < 60 {
		t.Fatalf("only %d of 63 pictures carved from the whole drive", all)
	}

	v := &vanishing{Device: drive}
	v.budget.Store(drive.Size() / 2)
	start := time.Now()
	res, err := Run(context.Background(), v, Options{Mode: Deep}, nil)
	if !disk.IsGone(err) {
		t.Fatalf("got %v, want the drive gone", err)
	}
	if took := time.Since(start); took > 10*time.Second {
		t.Fatalf("took %v to notice", took)
	}
	if len(res.Bad) != 0 {
		t.Fatalf("a missing drive counted as unreadable: %v", res.Bad)
	}
	if n := carvedFiles(res); n == 0 || n >= all {
		t.Fatalf("%d pictures kept from before the drive went, of %d", n, all)
	}
}
