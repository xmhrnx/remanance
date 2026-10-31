//go:build !android && !ios

// Package adbphones offers the Android phones connected to this computer
// by USB, through ADB, for a scan, listed with the drives (see
// engine/adb): each phone's shared storage and, on a rooted phone, its
// apps' databases.
package adbphones

import (
	"fmt"
	"strings"

	"github.com/xmhrnx/remanance/engine/adb"
	"github.com/xmhrnx/remanance/engine/disk"
)

// prefix starts every phone's ID.
const prefix = "adb:"

// Register adds the phones to disk.List.
func Register() { disk.Register(provider{}) }

type provider struct{}

// List lists the phones; without adb installed, none.
func (provider) List() ([]disk.Info, error) {
	devs, err := adb.Devices()
	if err != nil {
		return nil, nil
	}
	var out []disk.Info
	for _, d := range devs {
		if d.State != "device" && d.State != "unauthorized" {
			continue
		}
		out = append(out, adb.StorageInfo(d))
		if d.State == "device" && adb.HasSU(d) {
			out = append(out, adb.AppDataInfo(d))
		}
	}
	return out, nil
}

func (provider) Handles(id string) bool { return strings.HasPrefix(id, prefix) }

func (provider) Open(info disk.Info) (disk.Device, error) {
	rest := strings.TrimPrefix(info.ID, prefix)
	i := strings.LastIndexByte(rest, ':')
	if i < 0 {
		return nil, fmt.Errorf("no phone %s", info.ID)
	}
	serial, what := rest[:i], rest[i+1:]
	devs, err := adb.Devices()
	if err != nil {
		return nil, err
	}
	for _, d := range devs {
		if d.Serial != serial {
			continue
		}
		if what == "appdata" {
			return adb.OpenAppData(d)
		}
		return adb.OpenStorage(d)
	}
	return nil, fmt.Errorf("the phone was disconnected: connect it again and refresh")
}
