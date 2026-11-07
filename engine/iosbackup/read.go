package iosbackup

import (
	"os"
	"path/filepath"
	"time"

	"github.com/xmhrnx/remanance/engine/plist"
)

// Read describes the backup in dir, from its Info.plist and
// Manifest.plist.
func Read(dir string) (Backup, error) {
	b := Backup{Dir: dir}
	data, err := os.ReadFile(filepath.Join(dir, "Info.plist"))
	if err != nil {
		return b, err
	}
	v, err := plist.Parse(data)
	if err != nil {
		return b, err
	}
	info := plist.Dict(v)
	b.DeviceName = plist.String(info["Device Name"])
	if b.DeviceName == "" {
		b.DeviceName = plist.String(info["Display Name"])
	}
	b.Product = plist.String(info["Product Name"])
	if b.Product == "" {
		b.Product = plist.String(info["Product Type"])
	}
	b.IOS = plist.String(info["Product Version"])
	b.Date, _ = info["Last Backup Date"].(time.Time)
	if data, err := os.ReadFile(filepath.Join(dir, "Manifest.plist")); err == nil {
		if v, err := plist.Parse(data); err == nil {
			b.Encrypted, _ = plist.Dict(v)["IsEncrypted"].(bool)
		}
	}
	return b, nil
}

// ReadPicked reads the backup in dir or, given a folder of backups such
// as MobileSync/Backup, the newest one in it.
func ReadPicked(dir string) (Backup, error) {
	if b, err := Read(dir); err == nil {
		return b, nil
	}
	var newest Backup
	dirs, _ := os.ReadDir(dir)
	for _, d := range dirs {
		if b, err := Read(filepath.Join(dir, d.Name())); err == nil && (newest.Dir == "" || b.Date.After(newest.Date)) {
			newest = b
		}
	}
	if newest.Dir == "" {
		return newest, ErrNotBackup
	}
	return newest, nil
}
