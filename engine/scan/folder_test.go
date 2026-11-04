package scan

import (
	"context"
	"errors"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/xmhrnx/remanance/engine/disk"
	"github.com/xmhrnx/remanance/engine/fsys"
)

func dir(parent *fsys.Entry, name string, state fsys.State) *fsys.Entry {
	e := &fsys.Entry{Name: name, IsDir: true, State: state, Parent: parent}
	parent.Children = append(parent.Children, e)
	return e
}

func file(parent *fsys.Entry, name string, state fsys.State) {
	parent.Children = append(parent.Children, &fsys.Entry{Name: name, State: state, Parent: parent})
}

func TestKeepFolder(t *testing.T) {
	v := fsys.NewVolume("NTFS", 0, 1<<30)
	users := dir(v.Root, "Users", fsys.Existing)
	me := dir(users, "me", fsys.Existing)
	dir(me, "documents", fsys.Deleted) // an older folder of the same name
	docs := dir(me, "Documents", fsys.Existing)
	file(docs, "report.docx", fsys.Deleted)
	file(dir(me, "Music", fsys.Existing), "song.mp3", fsys.Deleted)
	file(v.Lost, "orphan.jpg", fsys.Orphan)

	if err := keepFolder(v, "users/me/documents"); err != nil {
		t.Fatal(err)
	}
	if v.Root != docs {
		t.Fatalf("root is %q, want the Documents that exists", v.Root.Name)
	}
	if st := v.Stats(); st.Files != 1 || st.Deleted != 1 {
		t.Errorf("stats %+v, want only report.docx", st)
	}
	if v.Root.Parent != me {
		t.Error("the folder lost its parents, and so its path")
	}

	v2 := fsys.NewVolume("NTFS", 0, 1<<30)
	if err := keepFolder(v2, "Nowhere"); !errors.Is(err, ErrNoFolder) {
		t.Errorf("a missing folder gave %v", err)
	}
}

// TestScanFolder scans a file system on its own, as a drive letter is,
// limited to one folder.
func TestScanFolder(t *testing.T) {
	for _, name := range []string{"ntfs", "fat32"} {
		t.Run(name, func(t *testing.T) {
			dev, err := disk.OpenImage(filepath.Join(fixtures, name+".img"))
			if err != nil {
				t.Skipf("no image %s: %v", name, err)
			}
			defer dev.Close()
			res, err := Run(context.Background(), dev, Options{Mode: Deep, Folder: "projects/src"}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Volumes) != 1 || res.Volumes[0].Volume == nil {
				t.Fatalf("volumes %+v", res.Volumes)
			}
			if res.Carved != nil && len(res.Carved.Root.Children) > 0 {
				t.Error("a folder scan carved by signature")
			}
			var names []string
			res.Volumes[0].Volume.Root.Walk(func(e *fsys.Entry) bool {
				if !e.IsDir {
					names = append(names, e.Name)
				}
				return true
			})
			sort.Strings(names)
			if strings.Join(names, ",") != "deepest.txt,main.go" {
				t.Errorf("found %v, want only what's in projects/src", names)
			}
		})
	}
}
