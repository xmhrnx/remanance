package phonestore

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/xmhrnx/remanance/engine/fsys"
	"github.com/xmhrnx/remanance/engine/scan"
)

func TestStore(t *testing.T) {
	root := t.TempDir()
	var pic bytes.Buffer
	png.Encode(&pic, image.NewGray(image.Rect(0, 0, 32, 32)))
	// A thumbnail cache: pictures packed into one file.
	cache := bytes.Repeat([]byte{0x55}, 8192)
	cache = append(cache, pic.Bytes()...)
	cache = append(cache, bytes.Repeat([]byte{0x55}, 8192)...)
	for rel, data := range map[string][]byte{
		"DCIM/Camera/IMG_1.jpg":                     []byte("in plain sight"),
		"DCIM/Camera/.trashed-1793404800-IMG_2.jpg": []byte("in the trash"),
		"DCIM/Camera/.pending-1793404800-IMG_3.jpg": []byte("being written"),
		"Android/media/com.whatsapp/Sent/.nomedia":  []byte("x"),
		"Android/media/com.whatsapp/Sent/IMG-1.jpg": []byte("sent copy"),
		"Pictures/.thumbnails/1234.jpg":             []byte("a thumbnail"),
		"DCIM/.thumbnails/.thumbdata4--1967290299":  cache,
		"Android/data/com.app/files/photo.jpg":      []byte("in plain sight"),
		"Android/data/com.app/cache/thumb.jpg":      []byte("a cached copy"),
		"Android/obb/com.app/main.obb":              []byte("an app package"),
		"Download/report.pdf":                       []byte("in plain sight"),
	} {
		p := filepath.Join(root, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	d, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	var _ scan.Tree = d

	res, err := scan.Run(context.Background(), d, scan.Options{Mode: scan.Deep}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]*fsys.Entry{}
	for _, v := range res.Volumes {
		v.Volume.Root.Walk(func(e *fsys.Entry) bool {
			if !e.IsDir {
				got[v.Volume.Label+":"+e.Path()] = e
			}
			return true
		})
	}
	if len(got) != 4 {
		t.Fatalf("found %v", got)
	}
	if e := got["Trash:/DCIM/Camera/IMG_2.jpg"]; e == nil || e.State != fsys.Deleted || e.Note == "In the trash" {
		t.Fatalf("trash: %+v", e)
	}
	for _, p := range []string{"Hidden copies:/Android/media/com.whatsapp/Sent/IMG-1.jpg", "Hidden copies:/Pictures/.thumbnails/1234.jpg",
		"Hidden copies:/Android/data/com.app/cache/thumb.jpg"} {
		if e := got[p]; e == nil || e.State != fsys.Orphan {
			t.Fatalf("%s: %+v", p, e)
		}
	}
	e := got["Trash:/DCIM/Camera/IMG_2.jpg"]
	data := make([]byte, e.Size)
	d.ReadAt(data, e.Runs[0].Offset)
	if string(data) != "in the trash" {
		t.Fatalf("trash file reads %q", data)
	}
	if res.Carved == nil || res.Carved.Stats().Files != 1 {
		t.Fatalf("carved %v", res.Carved)
	}
}
