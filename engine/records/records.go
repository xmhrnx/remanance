// Package records recovers deleted messages, calls, contacts, notes and
// other rows from the SQLite databases apps keep them in. A database
// keeps a deleted row's bytes until it needs the space (see
// sqlite.DB.Deleted), so a phone's messages can come back long after
// the files of its photos are gone: this is what phone recovery tools
// find in a backup. Each table's rows found are written as a CSV file,
// in a folder added to the volume.
package records

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/xmhrnx/remanance/engine/fsys"
	"github.com/xmhrnx/remanance/engine/sqlite"
)

// Folder is the name of the folder Find adds.
const Folder = "Deleted messages and records"

// maxSize is the largest database Find reads.
const maxSize = 1 << 30

// Find looks through the databases on v for deleted rows, and adds a
// Folder to v with what it finds, if anything. With all, it reads every
// SQLite database; without, only the ones it knows, such as an iPhone's
// messages, which suits a whole disk's thousands of app caches.
func Find(ctx context.Context, dev io.ReaderAt, v *fsys.Volume, all bool, progress func(done float64)) error {
	var dbs []*fsys.Entry
	v.Root.Walk(func(e *fsys.Entry) bool {
		if e.State == fsys.Existing && !e.IsDir && e.Size >= 1024 && e.Size <= maxSize && (known(e) != nil || all && looksLikeDB(e.Name)) {
			dbs = append(dbs, e)
		}
		return true
	})
	if len(dbs) == 0 {
		return nil
	}
	out := &fsys.Entry{Name: Folder, IsDir: true, Health: 100, State: fsys.Deleted}
	var id uint64 = 1 << 62 // clear of the volume's own IDs
	for i, e := range dbs {
		if err := ctx.Err(); err != nil {
			return err
		}
		if progress != nil {
			progress(float64(i) / float64(len(dbs)))
		}
		files := read(dev, e)
		if len(files) == 0 {
			continue
		}
		dir := folder(out, appName(e))
		for _, f := range files {
			id++
			f.ID = id
			f.Name = unique(dir, f.Name)
			dir.Add(f)
		}
	}
	if len(out.Children) == 0 {
		return nil
	}
	v.Root.Add(out)
	v.Sort()
	return nil
}

// looksLikeDB tells whether a file's name is one SQLite databases have.
func looksLikeDB(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".db", ".sqlite", ".sqlite3", ".sqlitedb", ".storedata":
		return true
	}
	return false
}

// appName is the folder a database's rows go in: the app's name for
// those Find knows, its path for others.
func appName(e *fsys.Entry) string {
	if v := known(e); v != nil {
		return v.app
	}
	return strings.ReplaceAll(strings.TrimPrefix(e.Path(), "/"), "/", " › ")
}

// read finds the deleted rows of the database e, as CSV files, one per
// table. A database that won't open, or has nothing deleted, gives
// none.
func read(dev io.ReaderAt, e *fsys.Entry) (files []*fsys.Entry) {
	defer func() {
		if r := recover(); r != nil {
			files = nil // a database too damaged to read
		}
	}()
	r, err := fsys.Open(dev, e)
	if err != nil {
		return nil
	}
	db, err := sqlite.Open(r, e.Size)
	if err != nil {
		return nil
	}
	if w := wal(e); w != nil {
		if wr, err := fsys.Open(dev, w); err == nil {
			db.UseWAL(wr, w.Size)
		}
	}
	recs, err := db.Deleted()
	if err != nil || len(recs) == 0 {
		return nil
	}
	byTable := map[string][]sqlite.Record{}
	var order []string
	for _, r := range recs {
		if byTable[r.Table] == nil {
			order = append(order, r.Table)
		}
		byTable[r.Table] = append(byTable[r.Table], r)
	}
	views := known(e)
	for _, name := range order {
		t, err := db.Table(name)
		if err != nil {
			continue
		}
		rows := byTable[name]
		var head []string
		var body [][]string
		file := name + ".csv"
		if vw := views.find(name); vw != nil {
			file = vw.file
			head, body = vw.render(db, t, rows)
		} else {
			head, body = plain(t, rows)
		}
		data := writeCSV(head, body)
		files = append(files, &fsys.Entry{
			Name: file, Size: int64(len(data)), Inline: data, Health: 100, State: fsys.Deleted,
			Modified: e.Modified,
			Note:     fmt.Sprintf("%d deleted %s found in %s", len(rows), plural(len(rows), "row", "rows"), e.Path()),
		})
	}
	return files
}
