// Package iphone reads an iPhone or iPad connected by USB, through
// Apple's USB service (usbmuxd, which iTunes or Apple Devices installs on
// Windows): after the phone trusts this computer, its media folder,
// where the camera roll and the Photos library are, is read directly.
//
// The phone's storage is encrypted file by file, and a deleted file's
// key is erased with it, so its sectors can't be read back from any
// computer. What a scan of the media folder finds is:
//
//   - photos and videos in Recently Deleted, which iOS keeps for 30
//     days, marked from the Photos library;
//   - previews of deleted photos: iOS keeps a thumbnail of each photo
//     in PhotoData, and it often outlives the photo;
//   - deleted rows in the Photos library, which name deleted photos and
//     when they were taken (see engine/records);
//   - pictures inside the thumbnail caches, which a deep scan searches
//     by signature.
package iphone

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/xmhrnx/remanance/engine/disk"
	"github.com/xmhrnx/remanance/engine/plist"
)

// Phone is an iPhone or iPad connected by USB.
type Phone struct {
	UDID    string
	Name    string // "Sam's iPhone"
	Product string // "iPhone15,2"
	dev     device
}

// Media is a phone's media folder opened for scanning, as a scan.Tree.
type Media struct {
	*disk.FileSet
	a          *afc
	start, end int64 // what a deep scan searches: the caches
}

// file is a file in the media folder.
type file struct {
	path string // under the media folder, without a leading "/"
	size int64
	mod  time.Time
}

// trustTimeout is how long the phone's owner has to tap Trust.
const trustTimeout = 2 * time.Minute

// Connected lists the iPhones and iPads connected by USB.
func Connected() ([]Phone, error) {
	devs, err := devices()
	if err != nil {
		return nil, err
	}
	var out []Phone
	for _, d := range devs {
		p := Phone{UDID: d.udid, Name: "iPhone", dev: d}
		if l, err := openLockdown(d); err == nil {
			if n := plist.String(l.value("", "DeviceName")); n != "" {
				p.Name = n
			}
			p.Product = plist.String(l.value("", "ProductType"))
			l.Close()
		}
		out = append(out, p)
	}
	return out, nil
}

// MediaInfo describes a phone's media folder as a source to scan.
func MediaInfo(p Phone) disk.Info {
	return disk.Info{ID: "iphone:" + p.UDID, Path: p.UDID, Name: p.Name,
		Model: "Recently Deleted, previews of deleted photos and caches", Bus: "USB", Kind: disk.KindFiles}
}

// OpenMedia opens a phone's media folder. The first time, the phone asks
// its owner to trust this computer, and this waits for the answer.
func OpenMedia(p Phone) (*Media, error) {
	l, err := openLockdown(p.dev)
	if err != nil {
		return nil, err
	}
	defer l.Close()
	ctx, cancel := context.WithTimeout(context.Background(), trustTimeout)
	defer cancel()
	if err := l.pair(ctx, func() {}); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("the iPhone wasn't trusted in time: unlock it, tap Trust, enter its passcode and try again")
		}
		return nil, err
	}
	if err := l.startSession(); err != nil {
		return nil, err
	}
	c, err := l.startService("com.apple.afc", false)
	if err != nil {
		return nil, err
	}
	a := &afc{rw: c}
	m := &Media{a: a}
	m.FileSet = disk.NewFileSetWith(MediaInfo(p), func(p string) (disk.File, error) { return a.open(p) })
	return m, nil
}

func (m *Media) Close() error {
	err := m.FileSet.Close()
	m.a.rw.Close()
	return err
}

// CarveRange is where the caches are.
func (m *Media) CarveRange() (int64, int64) { return m.start, m.end }
