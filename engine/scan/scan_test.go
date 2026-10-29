package scan

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mehran9675/remanence/engine/disk"
	"github.com/mehran9675/remanence/engine/fsys"
	_ "github.com/mehran9675/remanence/engine/fsys/all"
)

// The images come from testdata/mkimages.sh.
var fixtures = filepath.Join("..", "..", "testdata", "fixtures")

type manifestFile struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

func manifest(t *testing.T, name string) []manifestFile {
	b, err := os.ReadFile(filepath.Join(fixtures, name+".json"))
	if err != nil {
		t.Skipf("no fixture %s (run testdata/mkimages.sh): %v", name, err)
	}
	var m []manifestFile
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func deepScan(t *testing.T, name string) (*Result, disk.Device) {
	dev, err := disk.OpenImage(filepath.Join(fixtures, name+".img"))
	if err != nil {
		t.Skipf("no image %s: %v", name, err)
	}
	t.Cleanup(func() { dev.Close() })
	res, err := Run(context.Background(), dev, Options{Mode: Deep}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return res, dev
}

// hashes returns the SHA-256 of every file the scan found, wherever it
// was found.
func hashes(t *testing.T, dev io.ReaderAt, res *Result) map[string]string {
	out := map[string]string{}
	add := func(v *fsys.Volume) {
		if v == nil {
			return
		}
		for _, root := range []*fsys.Entry{v.Root, v.Lost} {
			if root == nil {
				continue
			}
			root.Walk(func(e *fsys.Entry) bool {
				if e.IsDir || e.Size > 64<<20 {
					return true
				}
				r, err := fsys.Open(dev, e)
				if err != nil {
					return true
				}
				h := sha256.New()
				io.Copy(h, io.NewSectionReader(r, 0, e.Size))
				out[hex.EncodeToString(h.Sum(nil))] = e.Path()
				return true
			})
		}
	}
	for _, v := range res.Volumes {
		add(v.Volume)
	}
	add(res.Carved)
	return out
}

func TestLostPartitions(t *testing.T) {
	p1, p2 := manifest(t, "lostparts-p1"), manifest(t, "lostparts-p2")
	res, dev := deepScan(t, "lostparts")
	want := map[int64]string{2048 * 512: "NTFS", (2048 + 90*2048) * 512: "FAT32"}
	for _, v := range res.Volumes {
		if fs, ok := want[v.Partition.Offset]; ok && v.Partition.FS == fs && v.Volume != nil {
			if !v.Partition.Lost {
				t.Errorf("partition at %d not marked lost", v.Partition.Offset)
			}
			delete(want, v.Partition.Offset)
		}
	}
	for off, fs := range want {
		t.Errorf("lost %s partition at %d not found", fs, off)
	}
	found := hashes(t, dev, res)
	for _, m := range [][]manifestFile{p1, p2} {
		for _, f := range m {
			if _, ok := found[f.SHA256]; !ok && !deletedInFixture(f.Path) {
				t.Errorf("%s from a lost partition not recovered", f.Path)
			}
		}
	}
}

func TestReformatted(t *testing.T) {
	files := manifest(t, "reformatted")
	res, dev := deepScan(t, "reformatted")
	found := hashes(t, dev, res)
	// The new root folder overwrote the old one, so the files directly in
	// it are left with no name and no size; those without a signature
	// (random data, plain text) are out of reach of any tool, as are the
	// fragmented ones. Everything in subfolders comes back through the
	// orphan-folder search, the rest by signature.
	unreachable := map[string]bool{
		"/big.bin": true, "/filler.bin": true, "/tiny.txt": true,
		"/fragmented.bin": true, "/projects/contacts.db": true,
	}
	var ok int
	for _, f := range files {
		_, hit := found[f.SHA256]
		switch {
		case hit:
			ok++
		case !unreachable[f.Path]:
			t.Errorf("not recovered: %s (%d bytes)", f.Path, f.Size)
		}
	}
	t.Logf("recovered %d of %d files from the quick-formatted volume", ok, len(files))
}

// deletedInFixture mirrors the files mkimages.sh deletes.
func deletedInFixture(p string) bool {
	switch p {
	case "/docs/report.txt", "/photos/beach.jpg", "/big.bin", "/tiny.txt", "/fragmented.bin":
		return true
	}
	return len(p) > 10 && p[:10] == "/projects/"
}
