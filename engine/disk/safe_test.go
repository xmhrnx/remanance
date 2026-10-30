package disk

import (
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"testing"
	"time"
)

// flaky is a drive in memory with unreadable spans, that can disconnect.
type flaky struct {
	data   []byte
	bad    []Range
	goneAt int64 // reads at or past it fail as if unplugged; 0 never
	reads  atomic.Int64
	closed bool
	info   Info
}

var errMedium = errors.New("medium error")

func (f *flaky) ReadAt(p []byte, off int64) (int, error) {
	f.reads.Add(1)
	if f.goneAt > 0 && off+int64(len(p)) > f.goneAt {
		return 0, fmt.Errorf("read: %w", ErrGone)
	}
	for _, b := range f.bad {
		if off < b.Offset+b.Length && off+int64(len(p)) > b.Offset {
			return 0, errMedium
		}
	}
	if off >= int64(len(f.data)) {
		return 0, io.EOF
	}
	n := copy(p, f.data[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

func (f *flaky) Info() Info      { return f.info }
func (f *flaky) Size() int64     { return int64(len(f.data)) }
func (f *flaky) SectorSize() int { return 512 }
func (f *flaky) Close() error    { f.closed = true; return nil }

func TestSafeReaderCountsBadSectorsOnce(t *testing.T) {
	f := &flaky{data: make([]byte, 1<<20), bad: []Range{{4096, 1024}, {8192, 512}}}
	s := NewSafeReader(f, f.Size(), 512)
	buf := make([]byte, 64<<10)
	// The same area read three times, overlapping, as a scan does.
	for _, off := range []int64{0, 0, 2048} {
		if _, err := s.ReadAt(buf, off); err != nil {
			t.Fatal(err)
		}
	}
	if got := s.BadBytes(); got != 1536 {
		t.Fatalf("bad bytes %d, want 1536 (%v)", got, s.BadRanges())
	}
	if r := s.BadRanges(); len(r) != 2 || r[0] != (Range{4096, 1024}) || r[1] != (Range{8192, 512}) {
		t.Fatalf("ranges %v", r)
	}
}

func TestSafeReaderSkipsDeadAreas(t *testing.T) {
	// 4 MB unreadable: retrying every sector would take 8192 reads.
	f := &flaky{data: make([]byte, 8<<20), bad: []Range{{1 << 20, 4 << 20}}}
	s := NewSafeReader(f, f.Size(), 512)
	buf := make([]byte, 8<<20)
	if _, err := s.ReadAt(buf, 0); err != nil {
		t.Fatal(err)
	}
	if n := f.reads.Load(); n > 1+128+64*giveUpAfter {
		t.Fatalf("%d reads", n)
	}
	if got := s.BadBytes(); got != 4<<20 {
		t.Fatalf("bad bytes %d, want %d", got, 4<<20)
	}
}

func TestSafeReaderStopsWhenTheDriveIsGone(t *testing.T) {
	f := &flaky{data: make([]byte, 1<<20), goneAt: 256 << 10}
	s := NewSafeReader(f, f.Size(), 512)
	buf := make([]byte, 64<<10)
	if _, err := s.ReadAt(buf, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadAt(buf, 512<<10); !IsGone(err) {
		t.Fatalf("got %v, want the drive gone", err)
	}
	reads := f.reads.Load()
	if _, err := s.ReadAt(buf, 0); !IsGone(err) || f.reads.Load() != reads {
		t.Fatalf("read after the drive went: %v", err)
	}
	if s.BadBytes() != 0 {
		t.Fatalf("a missing drive counted as %d bad bytes", s.BadBytes())
	}
}

func TestReconnecting(t *testing.T) {
	defer func(w, p time.Duration) { reconnectWait, reconnectPoll = w, p }(reconnectWait, reconnectPoll)
	reconnectWait, reconnectPoll = 300*time.Millisecond, 10*time.Millisecond

	data := make([]byte, 1<<20)
	data[700_000] = 42
	info := Info{ID: `\\.\PhysicalDrive2`, Path: `\\.\PhysicalDrive2`, Serial: "AD123", Size: 1 << 20}
	old := &flaky{data: data, goneAt: 1, info: info}
	back := &flaky{data: data, info: info}
	back.info.Path = `\\.\PhysicalDrive3`

	var listed atomic.Int32
	r := Reconnecting(old).(*reconnecting)
	r.list = func() ([]Info, error) {
		// The drive shows up again on the third look.
		if listed.Add(1) < 3 {
			return nil, nil
		}
		return []Info{{Path: `\\.\PhysicalDrive1`, Serial: "OTHER", Size: 1 << 20}, back.info}, nil
	}
	r.open = func(i Info) (Device, error) {
		if i.Serial != "AD123" {
			t.Fatalf("opened the wrong drive %+v", i)
		}
		return back, nil
	}
	b := make([]byte, 1)
	if _, err := r.ReadAt(b, 700_000); err != nil || b[0] != 42 {
		t.Fatalf("read after reconnecting: %v %d", err, b[0])
	}
	if !old.closed {
		t.Fatal("the old handle wasn't closed")
	}
	if r.Info().Path != info.Path {
		t.Fatal("the drive's identity changed")
	}

	// A drive that doesn't come back fails the read, and the next read
	// doesn't wait again.
	back.goneAt = 1
	r.list = func() ([]Info, error) { return nil, nil }
	if _, err := r.ReadAt(b, 700_000); !IsGone(err) {
		t.Fatalf("got %v, want the drive gone", err)
	}
	start := time.Now()
	if _, err := r.ReadAt(b, 700_000); !IsGone(err) || time.Since(start) > reconnectWait/2 {
		t.Fatalf("second read: %v after %v", err, time.Since(start))
	}
}
