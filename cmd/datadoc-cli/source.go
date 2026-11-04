package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/xmhrnx/remanance/engine/disk"
	"github.com/xmhrnx/remanance/engine/iosbackup"
)

// openFolder opens the drive a folder is on, and says where on it the
// folder is.
func openFolder(dir string) (disk.Device, string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, "", err
	}
	infos, _ := disk.List()
	info, within, ok := disk.VolumeFor(infos, abs)
	if !ok {
		return nil, "", fmt.Errorf("%s isn't on a drive that can be read directly", dir)
	}
	dev, err := disk.Open(info)
	return dev, within, err
}

// openSource opens a drive by the path "list" shows, an image file, or
// an iPhone backup's folder.
func openSource(arg string) (disk.Device, error) {
	// An encrypted backup's password, kept out of the shell's history.
	password := os.Getenv("DATADOC_BACKUP_PASSWORD")
	infos, _ := disk.List()
	for _, i := range infos {
		if i.Path == arg || i.ID == arg {
			if password != "" && i.Kind == disk.KindFiles {
				iosbackup.SetPassword(i.Path, password)
			}
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
		if password != "" {
			iosbackup.SetPassword(b.Dir, password)
		}
		d, err := iosbackup.Open(b)
		if errors.Is(err, iosbackup.ErrPassword) {
			return nil, fmt.Errorf("%w; set DATADOC_BACKUP_PASSWORD to it", err)
		}
		return d, err
	}
	// A device the listing missed: try it directly.
	return disk.Open(disk.Info{ID: arg, Path: arg, Kind: disk.KindPhysical, Name: arg})
}
