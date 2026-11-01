package devsync

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func node(t *testing.T, name string) *Node {
	t.Helper()
	n, err := Open(t.TempDir(), name)
	if err != nil {
		t.Fatal(err)
	}
	if err := n.Start(context.Background(), Options{Port: -1, Rescan: 200 * time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { n.Close() })
	return n
}

// eventually waits for ok, nudging both devices along.
func eventually(t *testing.T, what string, ok func() bool, nodes ...*Node) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		for _, n := range nodes {
			n.SyncNow()
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func read(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return "<" + err.Error() + ">"
	}
	return string(b)
}

func TestSync(t *testing.T) {
	a, b := node(t, "Laptop"), node(t, "Phone")

	// The laptop adds the phone; the phone's owner accepts.
	if err := a.AddDevice(b.ID(), "Phone", b.Addr()); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the phone to be asked", func() bool { return len(b.Status().Pending) == 1 }, a, b)
	if p := b.Status().Pending[0]; p.Device != a.ID() || p.Name != "Laptop" {
		t.Fatalf("pending: %+v", p)
	}
	if err := b.AddDevice(a.ID(), "", a.Addr()); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the devices to connect", func() bool {
		s := a.Status()
		return len(s.Devices) == 1 && s.Devices[0].Connected
	}, a, b)

	// The laptop shares a folder; the phone accepts it.
	dirA, dirB := t.TempDir(), filepath.Join(t.TempDir(), "Photos")
	os.MkdirAll(filepath.Join(dirA, "2024"), 0o755)
	os.WriteFile(filepath.Join(dirA, "2024", "beach.jpg"), []byte(strings.Repeat("sand", 300000)), 0o644)
	os.WriteFile(filepath.Join(dirA, "notes.txt"), []byte("first"), 0o644)
	if _, err := a.AddFolder(Folder{Label: "Photos", Path: dirA, Devices: []string{b.ID()}}); err != nil {
		t.Fatal(err)
	}
	var offer Pending
	eventually(t, "the folder to be offered", func() bool {
		for _, p := range b.Status().Pending {
			if p.Folder != "" {
				offer = p
				return true
			}
		}
		return false
	}, a, b)
	if err := b.AcceptFolder(offer, dirB, TwoWay); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the files to arrive", func() bool {
		return read(filepath.Join(dirB, "2024", "beach.jpg")) == strings.Repeat("sand", 300000) &&
			read(filepath.Join(dirB, "notes.txt")) == "first"
	}, a, b)

	// A change on the phone comes back.
	time.Sleep(20 * time.Millisecond)
	os.WriteFile(filepath.Join(dirB, "notes.txt"), []byte("second"), 0o644)
	eventually(t, "the edit to come back", func() bool { return read(filepath.Join(dirA, "notes.txt")) == "second" }, a, b)

	// A deletion goes across.
	os.Remove(filepath.Join(dirA, "2024", "beach.jpg"))
	eventually(t, "the deletion to go across", func() bool {
		_, err := os.Stat(filepath.Join(dirB, "2024", "beach.jpg"))
		return os.IsNotExist(err)
	}, a, b)

	// Both change a file while apart: the later change wins, the other
	// is kept as a conflict copy.
	a.RemoveDevice(b.ID())
	eventually(t, "the devices to disconnect", func() bool {
		s := b.Status()
		return len(s.Devices) == 1 && !s.Devices[0].Connected
	}, a, b)
	os.WriteFile(filepath.Join(dirB, "notes.txt"), []byte("phone's edit"), 0o644)
	b.SyncNow()
	time.Sleep(300 * time.Millisecond)
	os.WriteFile(filepath.Join(dirA, "notes.txt"), []byte("laptop's edit, later"), 0o644)
	a.SyncNow()
	time.Sleep(300 * time.Millisecond)
	a.AddDevice(b.ID(), "Phone", b.Addr())
	cfg := a.Config()
	f := cfg.Folders[0]
	f.Devices = []string{b.ID()}
	a.UpdateFolder(f)
	eventually(t, "the conflict to settle", func() bool {
		if read(filepath.Join(dirB, "notes.txt")) != "laptop's edit, later" {
			return false
		}
		copies, _ := filepath.Glob(filepath.Join(dirB, "notes.sync-conflict-*.txt"))
		if len(copies) != 1 || read(copies[0]) != "phone's edit" {
			return false
		}
		// The copy travels to the laptop too.
		back, _ := filepath.Glob(filepath.Join(dirA, "notes.sync-conflict-*.txt"))
		return len(back) == 1
	}, a, b)
}

func TestVersion(t *testing.T) {
	v := Version{"a": 1}
	w := v.bump("BBBBBBBB-x")
	if v.compare(w) != older || w.compare(v) != newer || v.compare(v) != equal {
		t.Fatal("ordering")
	}
	x := v.bump("CCCCCCCC-x")
	if w.compare(x) != concurrent {
		t.Fatal("concurrent")
	}
	if m := w.merge(x); m.compare(w) != newer || m.compare(x) != newer {
		t.Fatal("merge")
	}
}

func TestValidName(t *testing.T) {
	for name, ok := range map[string]bool{"a/b.txt": true, "../x": false, "/etc/passwd": false, "a\\b": false,
		"a//b": false, ".remanence-tmp-x": false, "x/.remanence-versions/y": true, "ok": true} {
		if validName(name) != ok {
			t.Errorf("validName(%q) = %v", name, !ok)
		}
	}
	if NormalizeID("abcd efgh ijkl mnop qrst uvwx yz23 4567") != "ABCD-EFGH-IJKL-MNOP-QRST-UVWX-YZ23-4567" {
		t.Error("NormalizeID")
	}
}
