package recovery_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mehran9675/remanence/engine/disk"
	"github.com/mehran9675/remanence/engine/fsys"
	_ "github.com/mehran9675/remanence/engine/fsys/all"
	"github.com/mehran9675/remanence/engine/partition"
	"github.com/mehran9675/remanence/engine/recovery"
)

var fixtures = filepath.Join("..", "..", "testdata", "fixtures")

// TestRecoverDeleted copies every deleted file out of the exFAT fixture
// and checks each against what was written.
func TestRecoverDeleted(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(fixtures, "exfat.json"))
	if err != nil {
		t.Skipf("no fixture (run testdata/mkimages.sh): %v", err)
	}
	var want []struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
	}
	if err := json.Unmarshal(b, &want); err != nil {
		t.Fatal(err)
	}
	dev, err := disk.OpenImage(filepath.Join(fixtures, "exfat.img"))
	if err != nil {
		t.Skip(err)
	}
	defer dev.Close()
	pv, ok := partition.Probe(dev, 0)
	if !ok {
		t.Fatal("no file system recognized")
	}
	vol, err := fsys.ParserFor(pv.FS)(context.Background(), dev, 0, dev.Size(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var items []recovery.Item
	vol.Root.Walk(func(e *fsys.Entry) bool {
		if !e.IsDir && e.State != fsys.Existing {
			items = append(items, recovery.Item{Entry: e})
		}
		return true
	})
	out := t.TempDir()
	rep, err := recovery.Recover(context.Background(), dev, items, recovery.DirSink{Dir: out}, recovery.Options{KeepFolders: true, Report: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Failed != 0 || rep.Recovered != len(items) {
		t.Errorf("recovered %d of %d, %d failed", rep.Recovered, len(items), rep.Failed)
	}
	checked := 0
	for _, w := range want {
		data, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(w.Path)))
		if err != nil {
			continue // not deleted, so not recovered
		}
		checked++
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != w.SHA256 {
			t.Errorf("%s: recovered content differs", w.Path)
		}
	}
	if checked < 9 {
		t.Errorf("compared %d recovered files, want 9", checked)
	}
}
