package recovery

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mehran9675/remanence/engine/disk"
	"github.com/mehran9675/remanence/engine/fsys"
)

// unplugged fails every read the way a drive that dropped off the bus
// does.
type unplugged struct{}

func (unplugged) ReadAt([]byte, int64) (int, error) {
	return 0, fmt.Errorf("read: %w", disk.ErrGone)
}

func TestRecoverFromADisconnectedDrive(t *testing.T) {
	dir := t.TempDir()
	var items []Item
	for i := 0; i < 3; i++ {
		e := &fsys.Entry{Name: fmt.Sprintf("clip%d.mp4", i), Size: 4096, Health: 100, State: fsys.Carved,
			Runs: []fsys.Run{{Offset: int64(i) << 20, Length: 4096}}}
		items = append(items, Item{Entry: e})
	}
	rep, err := Recover(context.Background(), unplugged{}, items, DirSink{Dir: dir}, Options{Report: true}, nil)
	if !disk.IsGone(err) {
		t.Fatalf("got %v, want the drive gone", err)
	}
	if rep.Failed != 1 || len(rep.Outcomes) != 1 {
		t.Fatalf("failed %d, outcomes %d: it should stop at the first file", rep.Failed, len(rep.Outcomes))
	}
	names, _ := os.ReadDir(dir)
	if len(names) != 1 || names[0].Name() != "Recovery report.txt" {
		t.Fatalf("left behind %v", names)
	}
	report, _ := os.ReadFile(filepath.Join(dir, "Recovery report.txt"))
	if !bytes.Contains(report, []byte("disconnected")) || strings.Contains(string(report), "->") {
		t.Fatalf("report:\n%s", report)
	}
}
