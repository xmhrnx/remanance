package all

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mehran9675/remanence/engine/disk"
	"github.com/mehran9675/remanence/engine/fsys"
	"github.com/mehran9675/remanence/engine/partition"
)

// The images come from testdata/mkimages.sh.
var imageDir = filepath.Join("..", "..", "..", "testdata", "fixtures")

type manifestFile struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

func loadManifest(t *testing.T, name string) []manifestFile {
	b, err := os.ReadFile(filepath.Join(imageDir, name+".json"))
	if err != nil {
		t.Skipf("no fixture %s (run testdata/mkimages.sh): %v", name, err)
	}
	var m []manifestFile
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func isDeletedInFixture(p string) bool {
	switch p {
	case "/docs/report.txt", "/photos/beach.jpg", "/big.bin", "/tiny.txt", "/fragmented.bin":
		return true
	}
	return strings.HasPrefix(p, "/projects/")
}

func hashEntry(t *testing.T, dev io.ReaderAt, e *fsys.Entry) string {
	r, err := fsys.Open(dev, e)
	if err != nil {
		t.Errorf("%s: %v", e.Path(), err)
		return ""
	}
	h := sha256.New()
	if _, err := io.Copy(h, io.NewSectionReader(r, 0, e.Size)); err != nil {
		t.Errorf("%s: %v", e.Path(), err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func parseImage(t *testing.T, name string, deep bool) (*fsys.Volume, disk.Device) {
	dev, err := disk.OpenImage(filepath.Join(imageDir, name+".img"))
	if err != nil {
		t.Skipf("no image %s: %v", name, err)
	}
	t.Cleanup(func() { dev.Close() })
	v, ok := partition.Probe(dev, 0)
	if !ok {
		t.Fatalf("%s: no file system recognized", name)
	}
	parse := fsys.ParserFor(v.FS)
	if parse == nil {
		t.Fatalf("%s: no parser for %s", name, v.FS)
	}
	ctx := context.Background()
	if deep {
		ctx = fsys.WithDeep(ctx)
	}
	vol, err := parse(ctx, disk.NewSection(dev, 0, dev.Size()), 0, dev.Size(), nil)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return vol, dev
}

// checkFixture parses an image and compares what it finds with the files
// written into it. deletedMin is how many of the deleted files must come
// back byte for byte.
func checkFixture(t *testing.T, name string, deletedMin int) {
	files := loadManifest(t, name)
	vol, dev := parseImage(t, name, false)
	byPath := map[string]*fsys.Entry{}
	vol.Root.Walk(func(e *fsys.Entry) bool {
		if !e.IsDir {
			if old, dup := byPath[e.Path()]; !dup || old.State != fsys.Existing {
				byPath[e.Path()] = e
			}
		}
		return true
	})
	var deleted, deletedOK int
	for _, f := range files {
		e := byPath[f.Path]
		wasDeleted := isDeletedInFixture(f.Path)
		if wasDeleted {
			deleted++
		}
		if e == nil {
			if !wasDeleted {
				t.Errorf("%s: existing file %s not found", name, f.Path)
			} else {
				t.Logf("%s: deleted %s not found", name, f.Path)
			}
			continue
		}
		if e.Size != f.Size {
			if !wasDeleted {
				t.Errorf("%s: %s size %d, want %d", name, f.Path, e.Size, f.Size)
			} else {
				t.Logf("%s: deleted %s size %d, want %d (health %d, %s)", name, f.Path, e.Size, f.Size, e.Health, e.Note)
			}
			continue
		}
		got := hashEntry(t, dev, e)
		switch {
		case got == f.SHA256 && wasDeleted:
			deletedOK++
		case got != f.SHA256 && !wasDeleted:
			t.Errorf("%s: existing %s content differs", name, f.Path)
		case got != f.SHA256:
			t.Logf("%s: deleted %s content differs (health %d, %s)", name, f.Path, e.Health, e.Note)
		}
		if wasDeleted && e.State == fsys.Existing {
			t.Errorf("%s: deleted %s reported as existing", name, f.Path)
		}
	}
	st := vol.Stats()
	t.Logf("%s: %s %q, %d files, %d deleted listed; recovered %d of %d deleted files exactly; warnings %v",
		name, vol.FS, vol.Label, st.Files, st.Deleted, deletedOK, deleted, vol.Warnings)
	if deletedOK < deletedMin {
		t.Errorf("%s: recovered %d deleted files, want at least %d", name, deletedOK, deletedMin)
	}
}

// The minimums are what each format allows. Of the 9 deleted files, two
// are fragmented (fragmented.bin, and contacts.db, which SQLite grows
// alongside its journal): FAT forgets a deleted file's cluster chain, so
// no tool can put those back together. ext4 keeps the deleted names only
// in the journal, which may have wrapped past some of them.
func TestNTFS(t *testing.T)  { checkFixture(t, "ntfs", 9) }
func TestFAT32(t *testing.T) { checkFixture(t, "fat32", 7) }
func TestFAT16(t *testing.T) { checkFixture(t, "fat16", 7) }
func TestExFAT(t *testing.T) { checkFixture(t, "exfat", 9) }
func TestExt4(t *testing.T)  { checkFixture(t, "ext4", 8) }
func TestExt3(t *testing.T)  { checkFixture(t, "ext3", 9) }
