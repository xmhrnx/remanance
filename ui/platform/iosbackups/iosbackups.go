//go:build !android && !ios

// Package iosbackups offers the iPhone and iPad backups on this
// computer for a scan, listed with the drives (see engine/iosbackup).
package iosbackups

import (
	"strings"

	"github.com/xmhrnx/remanance/engine/disk"
	"github.com/xmhrnx/remanance/engine/iosbackup"
)

type provider struct{}

// prefix starts every backup's ID.
const prefix = "iosbackup:"

// Register adds the backups to disk.List.
func Register() { disk.Register(provider{}) }

func (provider) List() ([]disk.Info, error) {
	var out []disk.Info
	for _, b := range iosbackup.Find() {
		out = append(out, iosbackup.Info(b))
	}
	return out, nil
}

func (provider) Handles(id string) bool { return strings.HasPrefix(id, prefix) }

func (provider) Open(info disk.Info) (disk.Device, error) {
	b, err := iosbackup.Read(strings.TrimPrefix(info.ID, prefix))
	if err != nil {
		return nil, err
	}
	return iosbackup.Open(b)
}
