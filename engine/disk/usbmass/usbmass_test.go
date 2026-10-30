package usbmass

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/xmhrnx/remanance/engine/disk"
)

// target is a USB drive in memory: it parses CBWs, answers SCSI commands
// and stalls the way real drives do.
type target struct {
	disk      []byte
	blockSize int
	bad       map[uint64]bool // blocks that fail with a medium error
	notReady  int             // TEST UNIT READYs to fail first

	pending []byte // data then CSW waiting for BulkIn
	stall   bool   // the next BulkIn stalls
	csw     []byte
	sense   Sense
	resets  int
}

func (t *target) MaxTransfer() int { return 16 << 10 }
func (t *target) ClearHalt(bool) error {
	return nil
}
func (t *target) Reset() error { t.resets++; t.pending, t.csw, t.stall = nil, nil, false; return nil }

func (t *target) BulkOut(data []byte, _ time.Duration) (int, error) {
	le := binary.LittleEndian
	if len(data) != cbwLen || le.Uint32(data) != cbwSignature {
		return 0, errors.New("not a CBW")
	}
	tag, length := le.Uint32(data[4:]), int(le.Uint32(data[8:]))
	cdb := data[15 : 15+int(data[14])]
	reply, status := t.execute(cdb, length)
	t.pending = reply
	t.stall = status != statusPassed && length > 0
	csw := make([]byte, cswLen)
	le.PutUint32(csw, cswSignature)
	le.PutUint32(csw[4:], tag)
	le.PutUint32(csw[8:], uint32(length-len(reply)))
	csw[12] = byte(status)
	t.csw = csw
	return len(data), nil
}

func (t *target) BulkIn(data []byte, _ time.Duration) (int, error) {
	if t.stall {
		t.stall = false
		t.pending = nil
		return 0, ErrStall
	}
	if len(t.pending) > 0 {
		n := copy(data, t.pending)
		t.pending = t.pending[n:]
		return n, nil
	}
	if t.csw == nil {
		return 0, errors.New("nothing to read")
	}
	n := copy(data, t.csw)
	t.csw = nil
	return n, nil
}

func (t *target) fail(key, asc byte) ([]byte, int) {
	t.sense = Sense{Key: key, ASC: asc}
	return nil, statusFailed
}

func (t *target) execute(cdb []byte, length int) ([]byte, int) {
	be := binary.BigEndian
	blocks := uint64(len(t.disk) / t.blockSize)
	switch cdb[0] {
	case opTestUnitReady:
		if t.notReady > 0 {
			t.notReady--
			return t.fail(0x02, 0x04)
		}
		return nil, statusPassed
	case opRequestSense:
		r := make([]byte, 18)
		r[0], r[2], r[12], r[13] = 0x70, t.sense.Key, t.sense.ASC, t.sense.ASCQ
		return r[:min(length, 18)], statusPassed
	case opInquiry:
		r := make([]byte, 36)
		r[1] = 0x80
		copy(r[8:], "Generic ")
		copy(r[16:], "Flash Disk      ")
		return r[:min(length, 36)], statusPassed
	case opReadCapacity10:
		r := make([]byte, 8)
		be.PutUint32(r, uint32(blocks-1))
		be.PutUint32(r[4:], uint32(t.blockSize))
		return r, statusPassed
	case opRead10, opRead16:
		var lba uint64
		var count int
		if cdb[0] == opRead10 {
			lba, count = uint64(be.Uint32(cdb[2:])), int(be.Uint16(cdb[7:]))
		} else {
			lba, count = be.Uint64(cdb[2:]), int(be.Uint32(cdb[10:]))
		}
		if lba+uint64(count) > blocks {
			return t.fail(0x05, 0x21)
		}
		for b := lba; b < lba+uint64(count); b++ {
			if t.bad[b] {
				return t.fail(0x03, 0x11)
			}
		}
		off := int(lba) * t.blockSize
		return t.disk[off : off+count*t.blockSize], statusPassed
	}
	return t.fail(0x05, 0x20)
}

func newTarget(size, blockSize int) *target {
	data := make([]byte, size)
	rand.New(rand.NewSource(1)).Read(data)
	return &target{disk: data, blockSize: blockSize, bad: map[uint64]bool{}}
}

func TestOpenAndRead(t *testing.T) {
	tg := newTarget(4<<20, 512)
	tg.notReady = 3
	d, err := Open(tg, disk.Info{ID: "usb:1"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if d.Size() != 4<<20 || d.SectorSize() != 512 {
		t.Fatalf("size %d sector %d", d.Size(), d.SectorSize())
	}
	if info := d.Info(); info.Model != "Generic Flash Disk" || !info.Removable || info.Kind != disk.KindUSB {
		t.Fatalf("info %+v", info)
	}
	var _ disk.Device = d

	// Unaligned reads, and one bigger than a single READ.
	for _, c := range []struct{ off, n int }{{0, 512}, {3, 10}, {1000, 200 << 10}, {4<<20 - 700, 700}} {
		got := make([]byte, c.n)
		if _, err := d.ReadAt(got, int64(c.off)); err != nil {
			t.Fatalf("read %d+%d: %v", c.off, c.n, err)
		}
		if !bytes.Equal(got, tg.disk[c.off:c.off+c.n]) {
			t.Fatalf("read %d+%d: wrong data", c.off, c.n)
		}
	}
}

func TestBadSector(t *testing.T) {
	tg := newTarget(1<<20, 4096)
	tg.bad[10] = true
	d, err := Open(tg, disk.Info{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = d.ReadAt(make([]byte, 4096), 10*4096)
	var se *SenseError
	if !errors.As(err, &se) || !se.Medium() {
		t.Fatalf("got %v, want a medium error", err)
	}
	// The drive stays usable after the stall.
	got := make([]byte, 4096)
	if _, err := d.ReadAt(got, 11*4096); err != nil || !bytes.Equal(got, tg.disk[11*4096:12*4096]) {
		t.Fatalf("read after the error: %v", err)
	}
	if tg.resets != 0 {
		t.Fatalf("%d resets; a failed command needs none", tg.resets)
	}
}
