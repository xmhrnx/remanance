package sqlite

import (
	"bytes"
	"fmt"
	"os"
	"testing"
)

func open(t *testing.T) *DB {
	data, err := os.ReadFile("testdata/test.db")
	if err != nil {
		t.Fatal(err)
	}
	db, err := Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestFiles(t *testing.T) {
	tb, err := open(t).Table("files")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(tb.Columns) != "[fileID domain relativePath flags file]" {
		t.Fatalf("columns %v", tb.Columns)
	}
	path, blob := tb.Col("relativePath"), tb.Col("FILE")
	n := 0
	err = tb.Rows(func(_ int64, row []any) bool {
		if row[path] != fmt.Sprintf("Media/DCIM/100APPLE/IMG_%04d.HEIC", n) {
			t.Fatalf("row %d: %v", n, row[path])
		}
		if b, _ := row[blob].([]byte); len(b) != n%50 {
			t.Fatalf("row %d: blob of %d bytes", n, len(b))
		}
		n++
		return true
	})
	if err != nil || n != 600 {
		t.Fatalf("%d rows, %v", n, err)
	}
}

func TestNotes(t *testing.T) {
	tb, err := open(t).Table("notes")
	if err != nil {
		t.Fatal(err)
	}
	var rows [][]any
	tb.Rows(func(_ int64, row []any) bool { rows = append(rows, row); return true })
	if len(rows) != 2 {
		t.Fatalf("%d rows", len(rows))
	}
	r := rows[0]
	big, _ := r[3].([]byte)
	if r[0] != int64(1) || r[1] != "Café ☕" || r[2] != -1.5 || len(big) != 256*40 || big[300] != 300%256 || r[4] != nil {
		t.Fatalf("first row: %v %v %v %d bytes %v", r[0], r[1], r[2], len(big), r[4])
	}
	if rows[1][0] != int64(2) || rows[1][2] != int64(1<<40) || rows[1][3] != nil {
		t.Fatalf("second row: %v", rows[1])
	}
}

func TestDamaged(t *testing.T) {
	data, _ := os.ReadFile("testdata/test.db")
	for i := 100; i < len(data); i += 97 {
		bad := bytes.Clone(data)
		bad[i] ^= 0xa5
		db, err := Open(bytes.NewReader(bad), int64(len(bad)))
		if err != nil {
			continue
		}
		if tb, err := db.Table("Files"); err == nil {
			tb.Rows(func(int64, []any) bool { return true }) // must not panic
		}
	}
}
