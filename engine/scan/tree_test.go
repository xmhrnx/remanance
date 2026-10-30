package scan

import (
	"context"
	"testing"

	"github.com/xmhrnx/remanance/engine/fsys"
)

// fakeTree is a Tree over pictures(): one listed file, and the rest of
// the drive searched by signature.
type fakeTree struct{ *memDrive }

func (fakeTree) Volumes(context.Context, func(float64)) ([]*fsys.Volume, error) {
	v := fsys.NewVolume("Phone storage", 0, 0)
	v.Label = "Trash"
	v.Root.Add(&fsys.Entry{Name: "kept.png", Size: 100, State: fsys.Deleted, Health: 100,
		Runs: []fsys.Run{{Offset: 1 << 20, Length: 100}}})
	return []*fsys.Volume{v}, nil
}

func (fakeTree) CarveRange() (int64, int64) { return 0, 8 << 20 }

func TestTree(t *testing.T) {
	tree := fakeTree{pictures(t)}
	res, err := Run(context.Background(), tree, Options{Mode: Quick}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Volumes) != 1 || res.Volumes[0].Volume.Label != "Trash" || res.Carved != nil {
		t.Fatalf("quick scan: %+v", res)
	}
	res, err = Run(context.Background(), tree, Options{Mode: Deep}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// The PNGs at 2 to 7 MB; the one at 1 MB is the listed file's.
	if got := res.Carved.Stats().Files; got != 6 {
		t.Fatalf("carved %d files, want 6", got)
	}
}
