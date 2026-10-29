package main

import (
	"os"

	"github.com/mehran9675/remanence/engine/disk"
)

// openSource opens a drive by the path "list" shows, or an image file.
func openSource(arg string) (disk.Device, error) {
	infos, _ := disk.List()
	for _, i := range infos {
		if i.Path == arg || i.ID == arg {
			return disk.Open(i)
		}
	}
	if st, err := os.Stat(arg); err == nil && st.Mode().IsRegular() {
		return disk.OpenImage(arg)
	}
	// A device the listing missed: try it directly.
	return disk.Open(disk.Info{ID: arg, Path: arg, Kind: disk.KindPhysical, Name: arg})
}
