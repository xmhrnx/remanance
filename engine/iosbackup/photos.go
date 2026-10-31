package iosbackup

import (
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/xmhrnx/remanance/engine/disk"
	"github.com/xmhrnx/remanance/engine/fsys"
	"github.com/xmhrnx/remanance/engine/sqlite"
)

// photosDB is where the Photos app's library is, in its domain.
const photosDB = "CameraRollDomain/Media/PhotoData/Photos.sqlite"

// coreDataEpoch is when the Photos library's dates count from.
var coreDataEpoch = time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)

// markTrashed marks the photos and videos in Recently Deleted as
// deleted, from the Photos library in the backup. byPath has every file
// by its domain and path.
func markTrashed(set *disk.FileSet, byPath map[string]*fsys.Entry) {
	lib := byPath[photosDB]
	if lib == nil || len(lib.Runs) == 0 {
		return
	}
	media := map[string]*fsys.Entry{}
	for p, e := range byPath {
		if rest, ok := strings.CutPrefix(p, "CameraRollDomain/Media/"); ok {
			media[rest] = e
		}
	}
	MarkTrashed(disk.NewSection(set, lib.Runs[0].Offset, lib.Size), lib.Size, media)
}

// MarkTrashed marks the photos and videos in Recently Deleted as
// deleted, from the Photos library, Photos.sqlite. The files are still
// there: iOS keeps them for 30 days. media has the files by their path
// in the phone's media folder, such as "DCIM/100APPLE/IMG_0001.HEIC".
func MarkTrashed(lib io.ReaderAt, size int64, media map[string]*fsys.Entry) {
	db, err := sqlite.Open(lib, size)
	if err != nil {
		return
	}
	// ZASSET since iOS 14, ZGENERICASSET before.
	t, err := db.Table("ZASSET")
	if err != nil {
		if t, err = db.Table("ZGENERICASSET"); err != nil {
			return
		}
	}
	cDir, cName, cState, cDate := t.Col("ZDIRECTORY"), t.Col("ZFILENAME"), t.Col("ZTRASHEDSTATE"), t.Col("ZTRASHEDDATE")
	if min(cDir, cName, cState) < 0 {
		return
	}
	// A photo's files, such as the picture and a Live Photo's video,
	// share the name before the extension.
	byStem := map[string][]*fsys.Entry{}
	for p, e := range media {
		stem := strings.ToLower(strings.TrimSuffix(p, path.Ext(p)))
		byStem[stem] = append(byStem[stem], e)
	}
	t.Rows(func(_ int64, row []any) bool {
		if state, _ := row[cState].(int64); state != 1 {
			return true
		}
		dir, _ := row[cDir].(string)
		name, _ := row[cName].(string)
		note := "In Recently Deleted"
		if cDate >= 0 {
			// A REAL with no fraction may be stored as an integer.
			secs, _ := row[cDate].(float64)
			if n, ok := row[cDate].(int64); ok {
				secs = float64(n)
			}
			if secs > 0 {
				when := coreDataEpoch.Add(time.Duration(secs * float64(time.Second)))
				note += fmt.Sprintf(" since %s; iOS erases it for good 30 days after", when.Local().Format("Jan 2, 2006"))
			}
		}
		stem := strings.ToLower(path.Join(dir, strings.TrimSuffix(name, path.Ext(name))))
		for _, e := range byStem[stem] {
			e.State, e.Note = fsys.Deleted, note
		}
		return true
	})
}
