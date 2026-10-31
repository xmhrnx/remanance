package records

import (
	"bytes"
	"context"
	"encoding/csv"
	"os"
	"strings"
	"testing"

	"github.com/xmhrnx/remanance/engine/fsys"
)

// volume puts the files on one device, each starting on a 64 KB
// boundary, in a volume's folders.
func volume(t *testing.T, files map[string]string) (*fsys.Volume, *bytes.Reader) {
	var dev []byte
	v := fsys.NewVolume("test", 0, 0)
	dirs := map[string]*fsys.Entry{"": v.Root}
	for where, src := range files {
		data, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		off := int64(len(dev))
		dev = append(dev, data...)
		dev = append(dev, make([]byte, 65536-len(data)%65536)...)
		parent := v.Root
		parts := strings.Split(where, "/")
		for i := range parts[:len(parts)-1] {
			p := strings.Join(parts[:i+1], "/")
			if dirs[p] == nil {
				dirs[p] = &fsys.Entry{Name: parts[i], IsDir: true}
				parent.Add(dirs[p])
			}
			parent = dirs[p]
		}
		parent.Add(&fsys.Entry{Name: parts[len(parts)-1], Size: int64(len(data)), Runs: []fsys.Run{{Offset: off, Length: int64(len(data))}}})
	}
	return v, bytes.NewReader(dev)
}

func csvOf(t *testing.T, e *fsys.Entry) [][]string {
	rows, err := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(e.Inline, []byte(bom)))).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func child(e *fsys.Entry, name string) *fsys.Entry {
	for _, c := range e.Children {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestFind(t *testing.T) {
	v, dev := volume(t, map[string]string{
		"Phone/Library/SMS/sms.db":              "../sqlite/testdata/deleted.db",
		"Apps/com.example/Documents/app.db":     "../sqlite/testdata/wal.db",
		"Apps/com.example/Documents/app.db-wal": "../sqlite/testdata/wal.db-wal",
	})
	if err := Find(context.Background(), dev, v, true, nil); err != nil {
		t.Fatal(err)
	}
	out := child(v.Root, Folder)
	if out == nil {
		t.Fatal("no folder of deleted records")
	}
	msgs := child(child(out, "Messages"), "Messages.csv")
	if msgs == nil || msgs.State != fsys.Deleted {
		t.Fatalf("no Messages.csv in %v", out.Children)
	}
	rows := csvOf(t, msgs)
	if strings.Join(rows[0], ",") != "Date,Contact,Sent or received,Service,Text,Found in" {
		t.Fatalf("columns %v", rows[0])
	}
	var three []string
	for _, r := range rows[1:] {
		if strings.HasPrefix(r[4], "Message number 3:") {
			three = r
		}
	}
	if three == nil || three[1] != "sam@example.com" || three[2] != "Sent" || !strings.HasPrefix(three[0], "2023-") {
		t.Errorf("message 3: %v", three)
	}
	if len(rows) < 50 {
		t.Errorf("%d messages", len(rows)-1)
	}
	other := child(out, "Apps › com.example › Documents › app.db")
	if other == nil || child(other, "sms.csv") == nil {
		t.Fatalf("no rows of app.db: %v", out.Children)
	}
	if n := len(csvOf(t, child(other, "sms.csv"))) - 1; n != 14 {
		t.Errorf("%d rows of app.db, want 14", n)
	}
}

func TestTypedText(t *testing.T) {
	blob := append([]byte("\x04\x0bstreamtyped\x81\xe8\x03\x84\x01@\x84\x84\x84\x12NSAttributedString\x00\x84\x84\x08NSObject\x00\x85\x92\x84\x84\x84\x08NSString\x01\x94\x84\x01+\x0c"), "Hello there!"...)
	if s := typedText(blob); s != "Hello there!" {
		t.Errorf("%q", s)
	}
}
