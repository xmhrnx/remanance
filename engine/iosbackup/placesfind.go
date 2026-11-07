package iosbackup

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
)

// Places are where Finder and iTunes keep backups on this computer.
func Places() []string {
	return applePlaces()
}

func applePlaces() []string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "windows":
		return []string{
			filepath.Join(os.Getenv("APPDATA"), "Apple Computer", "MobileSync", "Backup"),
			// iTunes from the Microsoft Store.
			filepath.Join(home, "Apple", "MobileSync", "Backup"),
		}
	case "darwin":
		// Reading it needs Full Disk Access.
		return []string{filepath.Join(home, "Library", "Application Support", "MobileSync", "Backup")}
	}
	return nil
}

// Find lists the backups in Places, newest first.
func Find() []Backup {
	var out []Backup
	for _, place := range Places() {
		dirs, _ := os.ReadDir(place)
		for _, d := range dirs {
			if b, err := Read(filepath.Join(place, d.Name())); err == nil {
				out = append(out, b)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Date.After(out[j].Date) })
	return out
}
