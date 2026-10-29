package scan

import (
	"context"
	"fmt"
	"io"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mehran9675/remanence/engine/carve"
	"github.com/mehran9675/remanence/engine/disk"
	"github.com/mehran9675/remanence/engine/fsys"
	_ "github.com/mehran9675/remanence/engine/fsys/all"
	"github.com/mehran9675/remanence/engine/partition"
)

const blockSize = 4 << 20 // bytes per read in the sector search

// scanner holds a running scan's state.
type scanner struct {
	dev      disk.Device
	safe     *disk.SafeReader
	cached   io.ReaderAt
	opts     Options
	report   func(Progress)
	res      *Result
	sm       sectorMap
	started  time.Time
	files    atomic.Int64
	deleted  atomic.Int64
	carver   *carve.Scanner
	finder   *partition.Finder
	position atomic.Int64
	searchT0 time.Time

	mu       sync.Mutex
	phase    Phase
	fraction float64
	lastSent time.Time
}

// Run scans dev. onProgress, if set, is called from the scanning
// goroutine about ten times a second; it must not block. Cancelling ctx
// stops the scan and returns what was found so far with ctx's error.
func Run(ctx context.Context, dev disk.Device, opts Options, onProgress func(Progress)) (*Result, error) {
	s := &scanner{dev: dev, opts: opts, report: onProgress, started: time.Now()}
	s.safe = disk.NewSafeReader(dev, dev.Size(), dev.SectorSize())
	s.cached = disk.NewCache(s.safe, dev.Size(), 64<<10, 2048)
	s.sm.size = dev.Size()
	s.res = &Result{Device: dev.Info(), Mode: opts.Mode, Started: s.started}
	err := s.run(ctx)
	s.res.Bad = s.safe.BadRanges()
	s.res.Duration = time.Since(s.started)
	s.res.Map = s.sm.snapshot()
	s.setPhase(PhaseDone, 1)
	s.send(true)
	return s.res, err
}

func (s *scanner) run(ctx context.Context) error {
	s.setPhase(PhaseTable, 0)
	table, err := partition.Read(s.cached, s.dev.Size(), s.dev.SectorSize())
	if err != nil {
		return fmt.Errorf("reading the partition table: %w", err)
	}
	s.res.Table = table
	s.sm.mark(0, 64<<10, CellMetadata)

	fsShare := 0.9
	if s.opts.Mode == Deep {
		fsShare = 0.15
	}
	s.setPhase(PhaseFileSystem, 0.02)
	for i, p := range table.Partitions {
		if err := ctx.Err(); err != nil {
			return err
		}
		from := 0.02 + fsShare*float64(i)/float64(len(table.Partitions))
		to := 0.02 + fsShare*float64(i+1)/float64(len(table.Partitions))
		s.res.Volumes = append(s.res.Volumes, s.parse(ctx, p, from, to))
	}
	if s.opts.Mode != Deep {
		return nil
	}

	if err := s.search(ctx, 0.02+fsShare, 0.95); err != nil {
		return err
	}

	// Volumes the table doesn't list, found during the search.
	lost := s.finder.Lost(table.Partitions)
	s.setPhase(PhaseLost, 0.95)
	for i, p := range lost {
		if err := ctx.Err(); err != nil {
			return err
		}
		from := 0.95 + 0.04*float64(i)/float64(len(lost))
		s.res.Volumes = append(s.res.Volumes, s.parse(ctx, p, from, from+0.04/float64(len(lost))))
	}
	sortVolumes(s.res.Volumes)
	if s.carver != nil {
		s.res.Carved = carvedVolume(s.carver.Found(), s.dev.Size())
	}
	return nil
}

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

func (s *scanner) setPhase(p Phase, f float64) {
	s.mu.Lock()
	s.phase, s.fraction = p, f
	s.mu.Unlock()
	s.send(false)
}

func (s *scanner) send(force bool) {
	if s.report == nil {
		return
	}
	s.mu.Lock()
	if !force && time.Since(s.lastSent) < 100*time.Millisecond {
		s.mu.Unlock()
		return
	}
	s.lastSent = time.Now()
	p := Progress{
		Phase:    s.phase,
		Fraction: s.fraction,
		Position: s.position.Load(),
		Total:    s.searchEnd() - s.opts.Start,
		Files:    int(s.files.Load()),
		Deleted:  int(s.deleted.Load()),
		BadBytes: s.safe.BadBytes(),
		Map:      s.sm.snapshot(),
	}
	t0 := s.searchT0
	s.mu.Unlock()
	if s.carver != nil {
		p.Carved = s.carver.Count()
	}
	p.Partitions = len(s.res.Volumes)
	if !t0.IsZero() && p.Position > 0 {
		el := time.Since(t0).Seconds()
		p.Speed = float64(p.Position) / el
		if p.Speed > 0 && p.Total > p.Position {
			p.Remaining = time.Duration(float64(p.Total-p.Position) / p.Speed * float64(time.Second))
		}
	}
	s.report(p)
}

func (s *scanner) searchEnd() int64 {
	if s.opts.End > 0 && s.opts.End < s.dev.Size() {
		return s.opts.End
	}
	return s.dev.Size()
}

// sortVolumes keeps volumes in device order, found ones among them.
func sortVolumes(v []*VolumeResult) {
	sort.SliceStable(v, func(i, j int) bool { return v[i].Partition.Offset < v[j].Partition.Offset })
}
