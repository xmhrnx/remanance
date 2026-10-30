package usbmass

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/mehran9675/remanence/engine/disk"
)

// maxRead keeps each READ within what every drive accepts; 64 KiB
// transfers are what most USB storage drivers use.
const maxRead = 64 << 10

// Device is a USB drive opened through a Transport.
type Device struct {
	mu        sync.Mutex
	s         scsi
	info      disk.Info
	blockSize int
	blocks    uint64
	reader    io.ReaderAt
	closer    io.Closer
}

// Open starts talking to the drive on t: it waits for it to be ready,
// names it and reads its size. info supplies what the USB descriptors
// already said; closer is closed with the device.
func Open(t Transport, info disk.Info, closer io.Closer) (*Device, error) {
	d := &Device{s: scsi{bot: bot{t: t}}, closer: closer}
	if err := d.s.waitReady(8 * time.Second); err != nil {
		return nil, fmt.Errorf("the drive isn't ready: %w", err)
	}
	vendor, product, removable, err := d.s.inquiry()
	if err == nil {
		if name := strings.TrimSpace(vendor + " " + product); name != "" && info.Model == "" {
			info.Model = name
		}
		info.Removable = info.Removable || removable
	}
	blocks, size, err := d.s.capacity()
	if err != nil {
		return nil, fmt.Errorf("reading the drive's size: %w", err)
	}
	if size == 0 || size%512 != 0 || size > 64<<10 {
		return nil, fmt.Errorf("the drive reports %d-byte sectors", size)
	}
	d.blocks, d.blockSize = blocks, int(size)
	info.Kind = disk.KindUSB
	info.Size = int64(blocks) * int64(size)
	info.SectorSize = int(size)
	if info.Bus == "" {
		info.Bus = "USB"
	}
	d.info = info
	d.reader = disk.Aligned(blockReader{d}, d.blockSize, info.Size)
	return d, nil
}

// blockReader reads whole blocks; Aligned turns it into a ReaderAt for
// any offset.
type blockReader struct{ d *Device }

func (b blockReader) ReadAt(p []byte, off int64) (int, error) {
	d := b.d
	d.mu.Lock()
	defer d.mu.Unlock()
	bs := int64(d.blockSize)
	done := 0
	for done < len(p) {
		lba := uint64((off + int64(done)) / bs)
		if lba >= d.blocks {
			return done, io.EOF
		}
		count := min(uint64((len(p)-done)/d.blockSize), uint64(max(1, maxRead/d.blockSize)), d.blocks-lba)
		if count == 0 {
			return done, io.ErrShortBuffer
		}
		want := int(count) * d.blockSize
		n, err := d.s.read(lba, uint32(count), p[done:done+want], d.blocks > 1<<32)
		done += n
		if err != nil {
			return done, err
		}
		if n < want {
			return done, io.ErrUnexpectedEOF
		}
	}
	return done, nil
}

func (d *Device) ReadAt(p []byte, off int64) (int, error) { return d.reader.ReadAt(p, off) }
func (d *Device) Info() disk.Info                         { return d.info }
func (d *Device) Size() int64                             { return d.info.Size }
func (d *Device) SectorSize() int                         { return d.blockSize }

func (d *Device) Close() error {
	if d.closer != nil {
		return d.closer.Close()
	}
	return nil
}
