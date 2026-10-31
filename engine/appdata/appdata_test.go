package appdata

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/xmhrnx/remanance/engine/fsys"
	"github.com/xmhrnx/remanance/engine/scan"
)

// fakeShell serves files of this computer as the phone's.
type fakeShell map[string]string // the phone's path: the file here

func (f fakeShell) Run(command string) (string, error) {
	if !strings.HasPrefix(command, "find ") {
		return "", fmt.Errorf("unexpected %q", command)
	}
	var b strings.Builder
	for phone, here := range f {
		st, err := os.Stat(here)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "%d %d %s\n", st.Size(), st.ModTime().Unix(), phone)
	}
	b.WriteString("garbage line\n")
	return b.String(), nil
}

func (f fakeShell) Read(path string, p []byte, off int64) error {
	if off%512 != 0 || len(p)%512 != 0 {
		return fmt.Errorf("unaligned read of %d at %d", len(p), off)
	}
	data, err := os.ReadFile(f[path])
	if err != nil {
		return err
	}
	clear(p)
	if off < int64(len(data)) {
		copy(p, data[off:])
	}
	return nil
}

func TestScan(t *testing.T) {
	sms := "/data/user_de/0/com.android.providers.telephony/databases/mmssms.db"
	sh := fakeShell{
		sms:          "../sqlite/testdata/wal.db",
		sms + "-wal": "../sqlite/testdata/wal.db-wal",
		"/data/data/com.example.notes/databases/notes.db": "../sqlite/testdata/deleted.db",
	}
	d := Open(Info("test:appdata", "Test phone"), sh)
	res, err := scan.Run(context.Background(), d, scan.Options{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var msgs, notes *fsys.Entry
	var db *fsys.Entry
	res.Volumes[0].Volume.Root.Walk(func(e *fsys.Entry) bool {
		switch {
		case e.Name == "Messages.csv" && e.Parent.Name == "Messages":
			msgs = e
		case e.Name == "message.csv":
			notes = e
		case e.Name == "mmssms.db":
			db = e
		}
		return true
	})
	if db == nil || db.Path() != "/com.android.providers.telephony/databases/mmssms.db" {
		t.Fatalf("mmssms.db: %v", db)
	}
	if msgs == nil {
		t.Fatal("no deleted texts")
	}
	if n := strings.Count(string(msgs.Inline), "about the trip to the lake"); n != 14 {
		t.Errorf("%d deleted texts, want 14", n)
	}
	if notes == nil || !strings.Contains(string(notes.Inline), "Message number 3:") {
		t.Errorf("rows of another app's database: %v", notes)
	}
}

func TestShown(t *testing.T) {
	for p, want := range map[string]string{
		"/data/data/com.whatsapp/databases/msgstore.db": "com.whatsapp/databases/msgstore.db",
		"/data/user_de/0/a/b.db":                         "a/b.db",
		"/data/media/0/x.db":                             "",
	} {
		if got := shown(p); got != want {
			t.Errorf("%s: %q", p, got)
		}
	}
}
