package disk

import (
	"fmt"
	"os"
	"path/filepath"
)

// imageDevice is a raw disk image (.img, .dd, .bin, .iso, .raw) or any
// other file treated as a sequence of sectors.
type imageDevice struct {
	f    *os.File
	info Info
}

// OpenImage opens a raw image file read-only.
func OpenImage(path string) (Device, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if st.IsDir() {
		f.Close()
		return nil, fmt.Errorf("disk: %s is a folder", path)
	}
	return &imageDevice{f: f, info: Info{
		ID:         "image:" + path,
		Path:       path,
		Name:       filepath.Base(path),
		Kind:       KindImage,
		Size:       st.Size(),
		SectorSize: 512,
	}}, nil
}

// NewReaderDevice wraps an already open source (a picked file on a phone,
// a test buffer) as a Device.
func NewReaderDevice(r interface {
	ReadAt(p []byte, off int64) (int, error)
}, size int64, name string) Device {
	return &readerDevice{r: r, info: Info{ID: "reader:" + name, Name: name, Kind: KindImage, Size: size, SectorSize: 512}}
}

func (d *imageDevice) ReadAt(p []byte, off int64) (int, error) { return d.f.ReadAt(p, off) }
func (d *imageDevice) Info() Info                              { return d.info }
func (d *imageDevice) Size() int64                             { return d.info.Size }
func (d *imageDevice) SectorSize() int                         { return d.info.SectorSize }
func (d *imageDevice) Close() error                            { return d.f.Close() }

type readerDevice struct {
	r interface {
		ReadAt(p []byte, off int64) (int, error)
	}
	info Info
}

func (d *readerDevice) ReadAt(p []byte, off int64) (int, error) { return d.r.ReadAt(p, off) }
func (d *readerDevice) Info() Info                              { return d.info }
func (d *readerDevice) Size() int64                             { return d.info.Size }
func (d *readerDevice) SectorSize() int                         { return d.info.SectorSize }
func (d *readerDevice) Close() error {
	if c, ok := d.r.(interface{ Close() error }); ok {
		return c.Close()
	}
	return nil
}
