//go:build !android && !ios

// Package iphones offers the iPhones and iPads connected to this computer
// by USB for a scan, listed with the drives (see engine/iphone).
package iphones

import (
	"fmt"
	"strings"

	"github.com/xmhrnx/remanance/engine/disk"
	"github.com/xmhrnx/remanance/engine/iphone"
)

// prefix starts every phone's ID.
const prefix = "iphone:"

// Register adds the phones to disk.List.
func Register() { disk.Register(provider{}) }

type provider struct{}

// List lists the phones; without Apple's USB service, none.
func (provider) List() ([]disk.Info, error) {
	phones, err := iphone.Connected()
	if err != nil {
		return nil, nil
	}
	var out []disk.Info
	for _, p := range phones {
		out = append(out, iphone.MediaInfo(p))
	}
	return out, nil
}

func (provider) Handles(id string) bool { return strings.HasPrefix(id, prefix) }

func (provider) Open(info disk.Info) (disk.Device, error) {
	phones, err := iphone.Connected()
	if err != nil {
		return nil, err
	}
	for _, p := range phones {
		if prefix+p.UDID == info.ID {
			return iphone.OpenMedia(p)
		}
	}
	return nil, fmt.Errorf("the iPhone was disconnected: connect it again and refresh")
}
