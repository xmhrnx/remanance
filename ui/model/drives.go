package model

import (
	"errors"
	"io"

	"github.com/mehran9675/remanence/engine/disk"
)

// LoadDrives lists the drives in the background.
func (m *Model) LoadDrives() {
	if m.DrivesLoading {
		return
	}
	m.DrivesLoading = true
	go func() {
		infos, err := disk.List()
		m.App.Post(func() {
			m.Drives, m.DrivesErr, m.DrivesLoading = infos, err, false
			if errors.Is(err, disk.ErrPermission) {
				m.NeedsAdmin = true
			}
		})
	}()
}

// Open opens a drive in the background, since it can wait on the user
// (Android asks before an app may use a USB drive), then runs then. If
// the drive can't be opened, OpenErr says why.
func (m *Model) Open(info disk.Info, then func()) {
	if m.Opening {
		return
	}
	m.Opening, m.OpenErr = true, nil
	go func() {
		dev, err := disk.Open(info)
		m.App.Post(func() {
			m.Opening = false
			if err != nil {
				m.OpenErr = err
				if errors.Is(err, disk.ErrPermission) {
					m.NeedsAdmin = true
				}
				return
			}
			m.use(dev)
			then()
		})
	}()
}

// OpenImage opens a disk image file picked by the user. A file that can
// be read at any offset is used directly; anything else is refused,
// since a scan reads all over the image.
func (m *Model) OpenImage(path string) bool {
	dev, err := disk.OpenImage(path)
	if err != nil {
		m.OpenErr = err
		return false
	}
	m.use(dev)
	return true
}

// OpenReader opens a picked image that has no path, as phones hand out.
func (m *Model) OpenReader(r io.ReaderAt, size int64, name string) {
	m.use(disk.NewReaderDevice(r, size, name))
}

func (m *Model) use(dev disk.Device) {
	m.Close()
	m.OpenErr = nil
	m.Dev = dev
	m.Source = dev.Info()
}
