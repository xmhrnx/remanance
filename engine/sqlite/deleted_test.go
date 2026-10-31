package sqlite

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
)

func openFile(t *testing.T, name string) *DB {
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	db, err := Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestDeleted(t *testing.T) {
	db := openFile(t, "deleted.db")
	recs, err := db.Deleted()
	if err != nil {
		t.Fatal(err)
	}
	msg, _ := db.Table("message")
	text := msg.Col("text")
	found := map[string]Record{}
	for _, r := range recs {
		if r.Table != "message" {
			t.Errorf("a %s row: %v", r.Table, r.Values)
			continue
		}
		s, _ := r.Values[text].(string)
		found[s] = r
	}
	gone, rowids := 0, 0
	for i := 1; i <= 300; i++ {
		s := fmt.Sprintf("Message number %d: see you at %d o'clock", i, i%12+1)
		r, ok := found[s]
		deleted := i%10 == 3 && i < 200 || i >= 200 && i <= 260
		switch {
		case ok && !deleted:
			t.Errorf("message %d is still there, but was found deleted", i)
		case !ok && deleted:
			gone++
		case ok:
			if r.Values[msg.Col("guid")] != fmt.Sprintf("G-%05d", i) || r.Values[msg.Col("handle_id")] != int64(i%2+1) {
				t.Errorf("message %d: %v", i, r.Values)
			}
			if r.Rowid != 0 {
				rowids++
				if r.Rowid != int64(i) || r.Values[0] != int64(i) {
					t.Errorf("message %d found as row %d: %v", i, r.Rowid, r.Values)
				}
			}
		}
	}
	// 20 deleted one by one and 61 together, of which SQLite wrote over
	// 200 to 226 when it rebalanced the table.
	if gone > 27 {
		t.Errorf("%d of 81 deleted messages not found", gone)
	}
	if rowids < 15 {
		t.Errorf("only %d found with their rowid", rowids)
	}
}

func TestDeletedWAL(t *testing.T) {
	db := openFile(t, "wal.db")
	wal, _ := os.ReadFile("testdata/wal.db-wal")
	if err := db.UseWAL(bytes.NewReader(wal), int64(len(wal))); err != nil {
		t.Fatal(err)
	}
	sms, err := db.Table("sms")
	if err != nil {
		t.Fatal(err)
	}
	live := 0
	sms.Rows(func(rowid int64, row []any) bool {
		live++
		if rowid%7 == 0 {
			t.Errorf("row %d is deleted in the log, but still read", rowid)
		}
		if rowid == 1 && row[4] != int64(5) {
			t.Errorf("row 1 reads %v, without the log's change", row)
		}
		return true
	})
	if live != 86 {
		t.Errorf("%d rows, want 86", live)
	}
	recs, err := db.Deleted()
	if err != nil {
		t.Fatal(err)
	}
	found := map[int64]bool{}
	for _, r := range recs {
		body, _ := r.Values[3].(string)
		var n int64
		fmt.Sscanf(body, "Text %d", &n)
		if n%7 != 0 {
			t.Errorf("row %d isn't deleted: %v (%s)", n, r.Values, r.Where)
		}
		if !strings.Contains(body, "lake") || r.Values[1] != fmt.Sprintf("+1555010%d", n%10) {
			t.Errorf("found %v", r.Values)
		}
		found[n] = true
	}
	if len(found) != 14 {
		t.Errorf("found %d of the 14 deleted rows", len(found))
	}
}

func TestDeletedDamaged(t *testing.T) {
	for _, name := range []string{"deleted.db", "wal.db"} {
		data, _ := os.ReadFile("testdata/" + name)
		wal, _ := os.ReadFile("testdata/wal.db-wal")
		for i := 0; i < len(data); i += 61 {
			bad := bytes.Clone(data)
			bad[i] ^= 0x5a
			bad[(i*7)%len(bad)] = 0xff
			db, err := Open(bytes.NewReader(bad), int64(len(bad)))
			if err != nil {
				continue
			}
			if name == "wal.db" {
				w := bytes.Clone(wal)
				w[i%len(w)] ^= 0x5a
				db.UseWAL(bytes.NewReader(w), int64(len(w)))
			}
			db.Deleted() // must not panic
		}
	}
}
