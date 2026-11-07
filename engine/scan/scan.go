package scan

import (
	"context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/xmhrnx/remanance/engine/carve"
	"github.com/xmhrnx/remanance/engine/disk"
	_ "github.com/xmhrnx/remanance/engine/fsys/all"
	"github.com/xmhrnx/remanance/engine/partition"
	"github.com/xmhrnx/remanance/engine/records"
)

// blockSize is the bytes per read in the sector search.
const blockSize = 4 << 20

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
	s.res = &Result{Device: dev.Info(), Mode: opts.Mode, Folder: opts.Folder, Started: s.started}
	var err error
	t, tree := dev.(Tree)
	if tree {
		err = s.runTree(ctx, t)
	} else {
		err = s.run(ctx)
	}
	if err == nil {
		// A phone's or a backup's every database; on other disks, only
		// those of the apps records knows, of the thousands there.
		s.records(ctx, tree)
		err = s.stopped(ctx)
	}
	// What the search carved stays, however the scan ended.
	if s.carver != nil && s.res.Carved == nil {
		s.res.Carved = carvedVolume(s.carver.Found(), s.dev.Size())
	}
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
		if err := s.stopped(ctx); err != nil {
			return err
		}
		from := 0.02 + fsShare*float64(i)/float64(len(table.Partitions))
		to := 0.02 + fsShare*float64(i+1)/float64(len(table.Partitions))
		s.res.Volumes = append(s.res.Volumes, s.parse(ctx, p, from, to))
	}
	if err := s.stopped(ctx); err != nil || s.opts.Mode != Deep || s.opts.Folder != "" {
		return err
	}

	if err := s.search(ctx, 0.02+fsShare, 0.95); err != nil {
		return err
	}

	// Volumes the table doesn't list, found during the search.
	lost := s.finder.Lost(table.Partitions)
	s.setPhase(PhaseLost, 0.95)
	for i, p := range lost {
		if err := s.stopped(ctx); err != nil {
			return err
		}
		from := 0.95 + 0.04*float64(i)/float64(len(lost))
		s.res.Volumes = append(s.res.Volumes, s.parse(ctx, p, from, from+0.04/float64(len(lost))))
	}
	sortVolumes(s.res.Volumes)
	return s.stopped(ctx)
}

// records adds the deleted messages and other rows found in the
// volumes' databases.
func (s *scanner) records(ctx context.Context, all bool) {
	from := s.fraction
	n := float64(len(s.res.Volumes))
	for i, v := range s.res.Volumes {
		if v.Volume == nil || ctx.Err() != nil {
			continue
		}
		s.setPhase(PhaseRecords, from)
		records.Find(ctx, s.cached, v.Volume, all, func(f float64) {
			s.setPhase(PhaseRecords, from+(0.995-from)*(float64(i)+f)/n)
		})
	}
}

// stopped returns why the scan has to end early: cancelled, or the drive
// disconnected, when the reads after it would only fail.
func (s *scanner) stopped(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.safe.Gone()
}
