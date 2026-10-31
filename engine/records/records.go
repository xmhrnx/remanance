// Package records recovers deleted messages, calls, contacts, notes and
// other rows from the SQLite databases apps keep them in. A database
// keeps a deleted row's bytes until it needs the space (see
// sqlite.DB.Deleted), so a phone's messages can come back long after
// the files of its photos are gone: this is what phone recovery tools
// find in a backup. Each table's rows found are written as a CSV file,
// in a folder added to the volume.
package records

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"path"
	"strings"
	"time"
	"unicode/utf8"

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

// unique numbers a file's name when dir has one by that name, which two
// databases of the same app can give: "Calls (2).csv".
func unique(dir *fsys.Entry, name string) string {
	taken := func(n string) bool {
		for _, c := range dir.Children {
			if strings.EqualFold(c.Name, n) {
				return true
			}
		}
		return false
	}
	ext := path.Ext(name)
	out := name
	for i := 2; taken(out); i++ {
		out = fmt.Sprintf("%s (%d)%s", strings.TrimSuffix(name, ext), i, ext)
	}
	return out
}

// looksLikeDB tells whether a file's name is one SQLite databases have.
func looksLikeDB(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".db", ".sqlite", ".sqlite3", ".sqlitedb", ".storedata":
		return true
	}
	return false
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

// wal finds the database's write-ahead log beside it.
func wal(e *fsys.Entry) *fsys.Entry {
	if e.Parent == nil {
		return nil
	}
	for _, c := range e.Parent.Children {
		if c.Name == e.Name+"-wal" && c.State == fsys.Existing && c.Size > 32 {
			return c
		}
	}
	return nil
}

// appName is the folder a database's rows go in: the app's name for
// those Find knows, its path for others.
func appName(e *fsys.Entry) string {
	if v := known(e); v != nil {
		return v.app
	}
	return strings.ReplaceAll(strings.TrimPrefix(e.Path(), "/"), "/", " › ")
}

// folder finds or adds the folder named name in dir.
func folder(dir *fsys.Entry, name string) *fsys.Entry {
	for _, c := range dir.Children {
		if c.IsDir && c.Name == name {
			return c
		}
	}
	f := &fsys.Entry{Name: name, IsDir: true, Health: 100, State: fsys.Deleted}
	dir.Add(f)
	return f
}

// plain lays out a table's rows found as they are, with a column for
// their rowid and one for where each was found.
func plain(t *sqlite.Table, rows []sqlite.Record) ([]string, [][]string) {
	head := append([]string{"rowid"}, t.Columns...)
	head = append(head, "Found in")
	var body [][]string
	for _, r := range rows {
		line := []string{rowid(r)}
		for i := range t.Columns {
			var v any
			if i < len(r.Values) {
				v = r.Values[i]
			}
			line = append(line, format(v))
		}
		body = append(body, append(line, r.Where))
	}
	return head, body
}

func rowid(r sqlite.Record) string {
	if r.Rowid == 0 {
		return ""
	}
	return fmt.Sprint(r.Rowid)
}

// format writes a value for a CSV cell; data that isn't text as hex.
func format(v any) string {
	switch v := v.(type) {
	case nil:
		return ""
	case string:
		return v
	case []byte:
		if utf8.Valid(v) && printable(string(v)) {
			return string(v)
		}
		if len(v) > 64 {
			return fmt.Sprintf("x'%x…' (%d bytes)", v[:64], len(v))
		}
		return fmt.Sprintf("x'%x'", v)
	case float64:
		return fmt.Sprintf("%g", v)
	}
	return fmt.Sprint(v)
}

func printable(s string) bool {
	for _, r := range s {
		if r < 0x20 && r != '\n' && r != '\r' && r != '\t' {
			return false
		}
	}
	return true
}

// bom is UTF-8's byte order mark.
const bom = "\xef\xbb\xbf"

// writeCSV writes rows as CSV with a byte order mark, which Excel needs
// to read UTF-8.
func writeCSV(head []string, body [][]string) []byte {
	var b bytes.Buffer
	b.WriteString(bom)
	w := csv.NewWriter(&b)
	w.Write(head)
	w.WriteAll(body)
	return b.Bytes()
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// Dates, as apps store them.

// appleEpoch is when Apple's dates count from.
var appleEpoch = time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)

// appleDate reads a date in seconds, or nanoseconds, since 2001.
func appleDate(v any) string {
	var t time.Time
	switch v := v.(type) {
	case int64:
		if v == 0 {
			return ""
		}
		if v > 1e12 || v < -1e12 {
			t = appleEpoch.Add(time.Duration(v))
		} else {
			t = appleEpoch.Add(time.Duration(v) * time.Second)
		}
	case float64:
		if v == 0 {
			return ""
		}
		t = appleEpoch.Add(time.Duration(v * float64(time.Second)))
	default:
		return format(v)
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

// unixMillis reads a date in milliseconds since 1970, as Android
// stores them.
func unixMillis(v any) string {
	ms, ok := v.(int64)
	if !ok {
		return format(v)
	}
	if ms == 0 {
		return ""
	}
	if ms < 1e11 {
		ms *= 1000 // seconds, in older tables
	}
	return time.UnixMilli(ms).Local().Format("2006-01-02 15:04:05")
}
