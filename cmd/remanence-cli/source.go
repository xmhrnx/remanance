package main

import (
	"os"

	"github.com/xmhrnx/remanance/engine/disk"
	"github.com/xmhrnx/remanance/engine/iosbackup"
)

// openSource opens a drive by the path "list" shows, an image file, or
// an iPhone backup's folder.
func openSource(arg string) (disk.Device, error) {
	infos, _ := disk.List()
	for _, i := range infos {
		if i.Path == arg || i.ID == arg {
			return disk.Open(i)
		}
	}
	if st, err := os.Stat(arg); err == nil && st.Mode().IsRegular() {
		return disk.OpenImage(arg)
	} else if err == nil && st.IsDir() {
		b, err := iosbackup.ReadPicked(arg)
		if err != nil {
			return nil, err
		}
		return iosbackup.Open(b)
	}
	// A device the listing missed: try it directly.
	return disk.Open(disk.Info{ID: arg, Path: arg, Kind: disk.KindPhysical, Name: arg})
}
