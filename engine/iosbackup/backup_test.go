package iosbackup

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/xmhrnx/remanance/engine/fsys"
	"github.com/xmhrnx/remanance/engine/scan"
)

const fixture = "testdata/backup/00008110-000A1B2C3D4E5F60"

func TestRead(t *testing.T) {
	b, err := Read(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if b.DeviceName != "Test iPhone" || b.Product != "iPhone 15" || b.IOS != "18.1" || b.Encrypted ||
		!b.Date.Equal(time.Date(2026, 10, 1, 9, 30, 0, 0, time.UTC)) {
		t.Fatalf("%+v", b)
	}
}

func TestOpen(t *testing.T) {
	b, _ := Read(fixture)
	d, err := Open(b)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	var _ scan.Tree = d
	files := map[string]*fsys.Entry{}
	d.vol.Root.Walk(func(e *fsys.Entry) bool {
		if !e.IsDir {
			files[e.Path()] = e
		}
		return true
	})
	if len(files) != 6 {
		t.Fatalf("%d files: %v", len(files), files)
	}
	for p, want := range map[string]string{
		"/Camera Roll/DCIM/100APPLE/IMG_0001.HEIC":       "photo one",
		"/Camera Roll/DCIM/100APPLE/IMG_0002.HEIC":       "photo two, deleted",
		"/Apps/net.whatsapp.WhatsApp/Documents/note.txt": "hello",
		"/Phone/Library/SMS/sms.db":                      "not really a database",
		"/Camera Roll/DCIM/100APPLE/IMG_0002.MOV":        "its live video",
	} {
		e := files[p]
		if e == nil {
			t.Fatalf("no %s", p)
		}
		got := make([]byte, e.Size)
		if _, err := d.ReadAt(got, e.Runs[0].Offset); err != nil && err != io.EOF {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Errorf("%s reads %q", p, got)
		}
	}
	// The photo in Recently Deleted and its Live Photo video.
	for _, p := range []string{"/Camera Roll/DCIM/100APPLE/IMG_0002.HEIC", "/Camera Roll/DCIM/100APPLE/IMG_0002.MOV"} {
		if e := files[p]; e.State != fsys.Deleted || !strings.HasPrefix(e.Note, "In Recently Deleted since") {
			t.Errorf("%s: %v %q", p, e.State, e.Note)
		}
	}
	if e := files["/Camera Roll/DCIM/100APPLE/IMG_0001.HEIC"]; e.State != fsys.Existing || e.Modified.Unix() != 1790000000 {
		t.Errorf("IMG_0001: %v %v", e.State, e.Modified)
	}

	res, err := scan.Run(context.Background(), d, scan.Options{Mode: scan.Deep}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if files, deleted, _ := res.Stats(); files != 6 || deleted != 2 {
		t.Fatalf("scan found %d files, %d deleted", files, deleted)
	}
}

const encrypted = "testdata/backup-encrypted/00008110-000E2C3D4E5F6071"

func TestEncrypted(t *testing.T) {
	b, err := Read(encrypted)
	if err != nil || !b.Encrypted {
		t.Fatalf("%+v %v", b, err)
	}
	if _, err := Open(b); err != ErrPassword {
		t.Fatalf("without a password: %v", err)
	}
	SetPassword(b.Dir, "1234")
	if _, err := Open(b); err != ErrWrongPassword {
		t.Fatalf("with the wrong password: %v", err)
	}
	SetPassword(b.Dir, "test password")
	d, err := Open(b)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	files := map[string]*fsys.Entry{}
	d.vol.Root.Walk(func(e *fsys.Entry) bool {
		if !e.IsDir {
			files[e.Path()] = e
		}
		return true
	})
	photo := files["/Camera Roll/DCIM/100APPLE/IMG_0001.HEIC"]
	if photo == nil {
		t.Fatalf("files: %v", files)
	}
	got := make([]byte, photo.Size)
	if _, err := d.ReadAt(got, photo.Runs[0].Offset); err != nil && err != io.EOF {
		t.Fatal(err)
	}
	if string(got) != "photo one, encrypted" {
		t.Errorf("photo reads %q", got)
	}
	sms := files["/Phone/Library/SMS/sms.db"]
	if sms == nil || sms.Size != 49152 {
		t.Fatalf("sms.db: %+v", sms)
	}

	// A scan finds the messages deleted from it.
	res, err := scan.Run(context.Background(), d, scan.Options{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var msgs *fsys.Entry
	res.Volumes[0].Volume.Root.Walk(func(e *fsys.Entry) bool {
		if e.Name == "Messages.csv" {
			msgs = e
		}
		return true
	})
	if msgs == nil || !strings.Contains(string(msgs.Inline), "Message number 3: see you at 4 o'clock") {
		t.Fatalf("no deleted messages found: %+v", msgs)
	}
}

func TestReadPicked(t *testing.T) {
	for _, dir := range []string{fixture, "testdata/backup"} {
		if b, err := ReadPicked(dir); err != nil || b.DeviceName != "Test iPhone" {
			t.Fatalf("%s: %+v %v", dir, b, err)
		}
	}
	if _, err := ReadPicked("testdata"); err != ErrNotBackup {
		t.Fatalf("testdata: %v", err)
	}
}
