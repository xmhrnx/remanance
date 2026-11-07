package scan

import (
	"context"
	"fmt"

	"github.com/xmhrnx/remanance/engine/disk"
	"github.com/xmhrnx/remanance/engine/fsys"
	"github.com/xmhrnx/remanance/engine/partition"
)

// parse reads the file system of one partition.
func (s *scanner) parse(ctx context.Context, p partition.Partition, from, to float64) *VolumeResult {
	vr := &VolumeResult{Partition: p}
	parser := fsys.ParserFor(p.FS)
	if parser == nil {
		if p.FS != "" {
			vr.Err = fmt.Errorf("%s volumes aren't supported yet; a deep scan finds their files by signature", p.FS)
		}
		return vr
	}
	pctx := ctx
	if s.opts.Mode == Deep {
		pctx = fsys.WithDeep(ctx)
	}
	sec := disk.NewSection(s.cached, p.Offset, p.Length)
	vol, err := func() (v *fsys.Volume, err error) {
		// A parser must never take the whole scan down with it: damaged
		// metadata can be arbitrarily odd.
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("the %s metadata is too damaged to read (%v)", p.FS, r)
			}
		}()
		return parser(pctx, sec, p.Offset, p.Length, func(f float64) {
			s.setPhase(PhaseFileSystem, from+(to-from)*f)
		})
	}()
	if err != nil {
		vr.Err = err
		return vr
	}
	if s.opts.Folder != "" {
		if err := keepFolder(vol, s.opts.Folder); err != nil {
			vr.Err = err
			return vr
		}
	}
	vr.Volume = vol
	st := vol.Stats()
	s.files.Add(int64(st.Files))
	s.deleted.Add(int64(st.Deleted))
	s.sm.mark(p.Offset, min(p.Length, 1<<20), CellMetadata)
	s.send(false)
	return vr
}

// knownStarts maps the device offset where each file's data starts to
// where it ends, for every file found through a file system: the carver
// skips them instead of listing them a second time.
func (s *scanner) knownStarts() map[int64]int64 {
	out := map[int64]int64{}
	add := func(e *fsys.Entry) bool {
		if !e.IsDir && len(e.Runs) > 0 && e.Runs[0].Offset >= 0 && e.Health > 0 {
			end := e.Runs[0].Offset + e.Runs[0].Length
			if len(e.Runs) == 1 {
				end = e.Runs[0].Offset + e.Size
			}
			out[e.Runs[0].Offset] = end
		}
		return true
	}
	for _, v := range s.res.Volumes {
		if v.Volume != nil {
			v.Volume.Root.Walk(add)
			v.Volume.Lost.Walk(add)
		}
	}
	return out
}
