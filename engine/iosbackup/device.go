package iosbackup

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/xmhrnx/remanance/engine/disk"
	"github.com/xmhrnx/remanance/engine/fsys"
	"github.com/xmhrnx/remanance/engine/plist"
	"github.com/xmhrnx/remanance/engine/sqlite"
)

// Device is a backup opened for scanning: every file it holds, laid out
// as one device (see disk.FileSet), with the backup's own folders.
type Device struct {
	*disk.FileSet
	vol *fsys.Volume
}

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

// fileMeta is what an MBFile tells besides dates.
type fileMeta struct {
	size int64  // -1 if not given
	key  []byte // the file's key, wrapped, in an encrypted backup
}

// meta fills in an entry's dates from the MBFile archived for it, and
// returns its size and key.
func meta(e *fsys.Entry, blob []byte) fileMeta {
	m := fileMeta{size: -1}
	v, err := plist.Parse(blob)
	if err != nil {
		return m
	}
	a := plist.Dict(v)
	objs, _ := a["$objects"].([]any)
	root := plist.Int(plist.Dict(a["$top"])["root"])
	if root <= 0 || root >= int64(len(objs)) {
		return m
	}
	f := plist.Dict(objs[root])
	if t := plist.Int(f["LastModified"]); t > 0 {
		e.Modified = time.Unix(t, 0)
	}
	if t := plist.Int(f["Birth"]); t > 0 {
		e.Created = time.Unix(t, 0)
	}
	if n, ok := f["Size"]; ok {
		m.size = plist.Int(n)
	}
	if ref, ok := f["EncryptionKey"].(plist.UID); ok && int(ref) < len(objs) {
		m.key, _ = plist.Dict(objs[ref])["NS.data"].([]byte)
	}
	return m
}

// unlock unwraps an encrypted backup's class keys with the password set
// for it, and Manifest.db's key with them.
func unlock(dir string) (map[uint32][]byte, []byte, error) {
	data, err := os.ReadFile(filepath.Join(dir, "Manifest.plist"))
	if err != nil {
		return nil, nil, err
	}
	v, err := plist.Parse(data)
	if err != nil {
		return nil, nil, err
	}
	mp := plist.Dict(v)
	bag, _ := mp["BackupKeyBag"].([]byte)
	mkey, _ := mp["ManifestKey"].([]byte)
	kb, err := parseKeybag(bag)
	if err != nil {
		return nil, nil, err
	}

	passwords.Lock()
	pw, set := passwords.pw[dir]
	classes := passwords.keys[dir]
	passwords.Unlock()
	if !set {
		return nil, nil, ErrPassword
	}
	if classes == nil {
		if classes, err = kb.unlock(pw); err == errWrong {
			return nil, nil, ErrWrongPassword
		} else if err != nil {
			return nil, nil, err
		}
		passwords.Lock()
		passwords.keys[dir] = classes
		passwords.Unlock()
	}
	key, err := fileKey(classes, mkey)
	if err != nil {
		return nil, nil, fmt.Errorf("the key to the backup's list of files: %w", err)
	}
	return classes, key, nil
}

// Volumes returns the backup's files, read when it was opened.
func (d *Device) Volumes(context.Context, func(float64)) ([]*fsys.Volume, error) {
	return []*fsys.Volume{d.vol}, nil
}

// CarveRange is empty: the backup holds whole files.
func (d *Device) CarveRange() (int64, int64) { return 0, 0 }

// place is the folder a domain's files are shown in.
func place(domain string) string {
	switch {
	case domain == "CameraRollDomain":
		return "Camera Roll"
	case domain == "MediaDomain":
		return "Media"
	case domain == "HomeDomain":
		return "Phone"
	case strings.HasPrefix(domain, "AppDomainGroup-"):
		return "Apps/Shared/" + strings.TrimPrefix(domain, "AppDomainGroup-")
	case strings.HasPrefix(domain, "AppDomainPlugin-"):
		return "Apps/Extensions/" + strings.TrimPrefix(domain, "AppDomainPlugin-")
	case strings.HasPrefix(domain, "AppDomain-"):
		return "Apps/" + strings.TrimPrefix(domain, "AppDomain-")
	}
	return "System/" + domain
}

// shorten drops the "Media/" every Camera Roll path starts with.
func shorten(domain, rel string) string {
	if domain == "CameraRollDomain" {
		return strings.TrimPrefix(rel, "Media/")
	}
	return rel
}
