package iosbackup

import (
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
	return disk.Info{ID: "iosbackup:" + b.Dir, Path: b.Dir, Name: name, Model: model, Bus: "Backup", Kind: disk.KindFiles}
}

// Open reads the backup's list of files, Manifest.db, and opens it.
func Open(b Backup) (*Device, error) {
	if b.Encrypted {
		return nil, ErrEncrypted
	}
	mf := filepath.Join(b.Dir, "Manifest.db")
	f, err := os.Open(mf)
	if err != nil {
		if _, e := os.Stat(filepath.Join(b.Dir, "Manifest.mbdb")); e == nil {
			return nil, ErrOld
		}
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	db, err := sqlite.Open(f, st.Size())
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
		if cFile >= 0 {
			if blob, ok := row[cFile].([]byte); ok {
				meta(e, blob)
			}
		}
		e.Runs = []fsys.Run{{Offset: d.Add(src, st.Size()), Length: st.Size()}}
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

// meta fills in an entry's dates from the MBFile archived for it.
func meta(e *fsys.Entry, blob []byte) {
	v, err := plist.Parse(blob)
	if err != nil {
		return
	}
	a := plist.Dict(v)
	objs, _ := a["$objects"].([]any)
	root := plist.Int(plist.Dict(a["$top"])["root"])
	if root <= 0 || root >= int64(len(objs)) {
		return
	}
	f := plist.Dict(objs[root])
	if t := plist.Int(f["LastModified"]); t > 0 {
		e.Modified = time.Unix(t, 0)
	}
	if t := plist.Int(f["Birth"]); t > 0 {
		e.Created = time.Unix(t, 0)
	}
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
