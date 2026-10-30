package scan

import (
	"context"
	"fmt"

	"github.com/xmhrnx/remanance/engine/disk"
	"github.com/xmhrnx/remanance/engine/fsys"
	"github.com/xmhrnx/remanance/engine/partition"
)

// PhaseFiles is the step that lists a Tree's files.
const PhaseFiles Phase = "Looking through the files"

// Tree is a device made of files that knows its own file tree, such as
// a phone's shared storage or an iPhone backup (see disk.FileSet). A
// scan lists the tree instead of reading a partition table, and a deep
// scan also searches part of the device for files by signature.
type Tree interface {
	disk.Device
	// Volumes lists what the files hold, as one or more volumes, each a
	// folder of the results.
	Volumes(ctx context.Context, progress func(done float64)) ([]*fsys.Volume, error)
	// CarveRange is the stretch of the device a deep scan searches by
	// signature, such as caches with photos inside them; end 0 means
	// none.
	CarveRange() (start, end int64)
}

// runTree scans a Tree.
func (s *scanner) runTree(ctx context.Context, t Tree) error {
	share := 0.9
	if s.opts.Mode == Deep {
		share = 0.15
	}
	s.setPhase(PhaseFiles, 0)
	vols, err := func() (v []*fsys.Volume, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("listing the files failed (%v)", r)
			}
		}()
		return t.Volumes(ctx, func(f float64) { s.setPhase(PhaseFiles, share*f) })
	}()
	if err != nil {
		return err
	}
	// A Tree may only know its size once it has listed its files.
	s.safe = disk.NewSafeReader(t, t.Size(), t.SectorSize())
	s.cached = disk.NewCache(s.safe, t.Size(), 64<<10, 2048)
	s.sm.size = t.Size()
	s.res.Device = t.Info()
	for i, v := range vols {
		p := partition.Partition{Index: i + 1, Scheme: partition.SchemeNone, Length: t.Size(), FS: v.FS, Label: v.Label}
		s.res.Volumes = append(s.res.Volumes, &VolumeResult{Partition: p, Volume: v})
		st := v.Stats()
		s.files.Add(int64(st.Files))
		s.deleted.Add(int64(st.Deleted))
	}
	// Known once the files are listed.
	start, end := t.CarveRange()
	if err := s.stopped(ctx); err != nil || s.opts.Mode != Deep || end <= start {
		return err
	}
	s.opts.Start, s.opts.End = start, end
	return s.search(ctx, share, 0.99)
}
