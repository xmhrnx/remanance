package screens

import (
	"context"
	"slices"

	"github.com/mehran9675/giode"
	"github.com/xmhrnx/remanance/engine/disk"
	"github.com/xmhrnx/remanance/engine/iosbackup"
	"github.com/xmhrnx/remanance/engine/scan"
	"github.com/xmhrnx/remanance/locales"
)

func (d *Drives) pickImage() {
	go func() {
		files, err := d.m.App.PickFiles(context.Background(), giode.PickOptions{Types: imageTypes})
		if err != nil || len(files) == 0 {
			return
		}
		f := files[0]
		d.m.App.Post(func() {
			defer f.Close()
			if f.Path == "" {
				d.pickErr = errNoPath
				return
			}
			dev, err := disk.OpenImage(f.Path)
			if err != nil {
				d.pickErr = err
				return
			}
			info := dev.Info()
			dev.Close()
			d.addImage(info)
		})
	}()
}

// pickBackup opens an iPhone backup from a folder the user picks: the
// backup itself, or the Backup folder of a copy kept elsewhere.
func (d *Drives) pickBackup() {
	opts := giode.FolderOptions{Title: locales.T("drives.choose_an_iphone_or_ipad_backup")}
	go func() {
		dir, err := d.m.App.PickFolder(context.Background(), opts)
		if err != nil || dir == "" {
			return
		}
		b, err := iosbackup.ReadPicked(dir)
		d.m.App.Post(func() {
			if err != nil {
				d.pickErr = err
				return
			}
			info := iosbackup.Info(b)
			d.selected, d.pickErr = info.ID, nil
			for _, i := range slices.Concat(d.backups, d.m.Drives) {
				if i.ID == info.ID {
					return
				}
			}
			d.backups = append(d.backups, info)
		})
	}()
}

func (d *Drives) addImage(info disk.Info) {
	for _, i := range d.images {
		if i.ID == info.ID {
			d.selected = info.ID
			return
		}
	}
	d.images = append(d.images, info)
	d.selected, d.pickErr = info.ID, nil
}

// pickFolder asks for a folder and scans what was deleted from it.
func (d *Drives) pickFolder() {
	go func() {
		dir, err := d.m.App.PickFolder(context.Background(), giode.FolderOptions{Title: locales.T("drives.choose_a_folder_to_scan")})
		if err != nil || dir == "" {
			return
		}
		d.m.App.Post(func() {
			mode := scan.Quick
			if d.mode.Selected() == 1 {
				mode = scan.Deep
			}
			d.m.ScanFolder(dir, mode)
		})
	}()
}

// AddImage lists a disk image, as when one is opened with the app, and
// selects it.
func (d *Drives) AddImage(path string) error {
	dev, err := disk.OpenImage(path)
	if err != nil {
		return err
	}
	info := dev.Info()
	dev.Close()
	d.addImage(info)
	return nil
}
