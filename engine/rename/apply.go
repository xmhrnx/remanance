package rename

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Apply renames the files. Each goes to a temporary name first and then
// to its new one, so two files can trade names. If a rename fails, the
// ones done so far are undone, and the error says which file failed.
func Apply(cs []Change) ([]Done, error) {
	for _, c := range cs {
		if c.Problem != "" {
			return nil, ErrProblems
		}
	}
	type step struct{ from, tmp, to string }
	var steps []step
	stamp := strconv.FormatInt(time.Now().UnixNano(), 36)
	for i, c := range cs {
		if c.Same() {
			continue
		}
		dir := filepath.Dir(c.Path)
		steps = append(steps, step{c.Path, filepath.Join(dir, fmt.Sprintf(".rename-%s-%d", stamp, i)), filepath.Join(dir, c.New)})
	}
	var done []Done
	undoTmp := func(k int) {
		for j := k - 1; j >= 0; j-- {
			os.Rename(steps[j].tmp, steps[j].from)
		}
	}
	for k, s := range steps {
		if err := os.Rename(s.from, s.tmp); err != nil {
			undoTmp(k)
			return nil, fmt.Errorf("renaming %s: %w", filepath.Base(s.from), err)
		}
	}
	for k, s := range steps {
		if err := os.Rename(s.tmp, s.to); err != nil {
			Undo(done)
			// Those not yet moved go back from their temporary names.
			for _, r := range steps[k:] {
				os.Rename(r.tmp, r.from)
			}
			return nil, fmt.Errorf("renaming %s: %w", filepath.Base(s.from), err)
		}
		done = append(done, Done{From: s.from, To: s.to})
	}
	return done, nil
}

// Undo puts back the names Apply changed, in two steps like Apply, and
// returns what failed.
func Undo(done []Done) error {
	cs := make([]Change, 0, len(done))
	for _, d := range done {
		cs = append(cs, Change{Path: d.To, New: filepath.Base(d.From)})
	}
	_, err := Apply(cs)
	return err
}
