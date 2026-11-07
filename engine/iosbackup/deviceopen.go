package iosbackup

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/xmhrnx/remanance/engine/disk"
	"github.com/xmhrnx/remanance/engine/fsys"
	"github.com/xmhrnx/remanance/engine/sqlite"
)

// Info describes a backup as a source to scan.
func Info(b Backup) disk.Info {
	name := b.DeviceName
	if name == "" {
		name = "iPhone"
	}
	model := "iPhone backup"
	if !b.Date.IsZero() {
		model = "Backed up " + b.Date.Local().Format("Jan 2, 2006")
	}
	if b.Encrypted {
		model += ", encrypted"
	}
	return disk.Info{ID: "iosbackup:" + b.Dir, Path: b.Dir, Name: name, Model: model, Bus: "Backup", Kind: disk.KindFiles}
}

// Open reads the backup's list of files, Manifest.db, and opens it. An
// encrypted backup needs its password set first; without it, Open
// returns ErrPassword.
func Open(b Backup) (*Device, error) {
	var classes map[uint32][]byte
	data, err := os.ReadFile(filepath.Join(b.Dir, "Manifest.db"))
	if err != nil {
		if _, e := os.Stat(filepath.Join(b.Dir, "Manifest.mbdb")); e == nil {
			return nil, ErrOld
		}
		return nil, err
	}
	if b.Encrypted {
		var mkey []byte
		if classes, mkey, err = unlock(b.Dir); err != nil {
			return nil, err
		}
		if data, err = decrypt(mkey, data); err != nil {
			return nil, fmt.Errorf("decrypting the backup's list of files: %w", err)
		}
	}
	db, err := sqlite.Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("reading the backup's list of files: %w", err)
	}
	files, err := db.Table("Files")
	if err != nil {
		return nil, fmt.Errorf("reading the backup's list of files: %w", err)
	}
	cID, cDomain, cPath, cFlags, cFile := files.Col("fileID"), files.Col("domain"), files.Col("relativePath"), files.Col("flags"), files.Col("file")
	if min(cID, cDomain, cPath, cFlags) < 0 {
		return nil, fmt.Errorf("the backup's list of files has columns %v", files.Columns)
	}

	d := &Device{FileSet: disk.NewFileSet(Info(b))}
	v := fsys.NewVolume("iPhone backup", 0, 0)
	v.Label = d.Info().Name
	d.vol = v
	folders := map[string]*fsys.Entry{"": v.Root}
	var folder func(p string) *fsys.Entry
	folder = func(p string) *fsys.Entry {
		if p == "." || p == "/" {
			return v.Root
		}
		if e, ok := folders[p]; ok {
			return e
		}
		e := &fsys.Entry{Name: path.Base(p), IsDir: true, Health: 100}
		folder(path.Dir(p)).Add(e)
		folders[p] = e
		return e
	}
	byPath := map[string]*fsys.Entry{}
	var id uint64
	err = files.Rows(func(_ int64, row []any) bool {
		flags, _ := row[cFlags].(int64)
		fileID, _ := row[cID].(string)
		domain, _ := row[cDomain].(string)
		rel, _ := row[cPath].(string)
		if flags != 1 || len(fileID) < 2 || rel == "" {
			return true // folders appear with their files; links hold no data
		}
		src := filepath.Join(b.Dir, fileID[:2], fileID)
		st, err := os.Stat(src)
		if err != nil {
			return true // listed, but not in this backup's folders
		}
		where := path.Join(place(domain), shorten(domain, rel))
		id++
		e := &fsys.Entry{ID: id, Name: path.Base(where), Size: st.Size(), Health: 100, Modified: st.ModTime()}
		var m fileMeta
		if cFile >= 0 {
			if blob, ok := row[cFile].([]byte); ok {
				m = meta(e, blob)
			}
		}
		off := int64(0)
		if b.Encrypted {
			key, err := fileKey(classes, m.key)
			if err != nil || m.size < 0 {
				return true // not decryptable: a file of the device's own keys
			}
			e.Size = min(m.size, st.Size())
			if off, err = d.AddEncrypted(src, e.Size, key); err != nil {
				return true
			}
		} else {
			off = d.Add(src, st.Size())
		}
		e.Runs = []fsys.Run{{Offset: off, Length: e.Size}}
		folder(path.Dir(where)).Add(e)
		byPath[domain+"/"+rel] = e
		return true
	})
	if err != nil {
		d.Close()
		return nil, fmt.Errorf("reading the backup's list of files: %w", err)
	}
	markTrashed(d.FileSet, byPath)
	v.Size = d.Size()
	v.Sort()
	return d, nil
}
