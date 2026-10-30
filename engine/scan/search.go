package scan

import (
	"context"
	"io"
	"time"

	"github.com/xmhrnx/remanance/engine/carve"
	"github.com/xmhrnx/remanance/engine/disk"
	"github.com/xmhrnx/remanance/engine/partition"
)

type block struct {
	pos  int64
	data []byte
	bad  bool
}

// search reads the device from start to end once, feeding every block to
// the lost-partition finder and the carver. Reading runs ahead in its own
// goroutine so the device never waits on the parsing.
func (s *scanner) search(ctx context.Context, from, to float64) error {
	// Ending the search early, however it ends, releases the reader.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	s.setPhase(PhaseSectors, from)
	start := s.opts.Start - s.opts.Start%int64(s.dev.SectorSize())
	end := s.searchEnd()
	s.finder = partition.NewFinder(s.cached, s.dev.Size())

	sigs := s.opts.Signatures
	if sigs == nil || len(sigs) > 0 {
		known := s.knownStarts()
		skip := func(off int64) (int64, bool) {
			end, ok := known[off]
			return end, ok
		}
		s.carver = carve.NewScanner(s.cached, s.dev.Size(), s.dev.SectorSize(), sigs, skip)
	}

	s.mu.Lock()
	s.searchT0 = time.Now()
	s.mu.Unlock()

	blocks := make(chan block, 4)
	free := make(chan []byte, 6)
	for i := 0; i < 6; i++ {
		free <- make([]byte, blockSize+1024)
	}
	go func() {
		defer close(blocks)
		for pos := start; pos < end; pos += blockSize {
			var buf []byte
			select {
			case buf = <-free:
			case <-ctx.Done():
				return
			}
			n := int(min(int64(blockSize+1024), end-pos))
			// The 1 KB overlap lets a superblock straddle blocks.
			badBefore := s.safe.BadBytes()
			m, err := s.safe.ReadAt(buf[:n], pos)
			if disk.IsGone(err) {
				return
			}
			if m < n {
				clear(buf[m:n])
			}
			select {
			case blocks <- block{pos: pos, data: buf[:n], bad: s.safe.BadBytes() > badBefore || (err != nil && err != io.EOF)}:
			case <-ctx.Done():
				return
			}
		}
	}()

	total := float64(end - start)
	for b := range blocks {
		data := b.data
		scanLen := min(len(data), blockSize)
		for i := 0; i+1024 <= len(data) && i < scanLen; i += 512 {
			s.finder.Sector(data[i:], b.pos+int64(i))
		}
		before := 0
		if s.carver != nil {
			before = s.carver.Count()
			s.carver.Block(data[:scanLen], b.pos)
		}
		cell := CellRead
		if b.bad {
			cell = CellBad
		}
		s.sm.mark(b.pos, int64(scanLen), cell)
		if s.carver != nil && s.carver.Count() > before {
			s.sm.mark(b.pos, int64(scanLen), CellFound)
		}
		s.position.Store(b.pos + int64(scanLen) - start)
		free <- data[:cap(data)]
		s.mu.Lock()
		s.phase, s.fraction = PhaseSectors, from+(to-from)*float64(b.pos+int64(scanLen)-start)/total
		s.mu.Unlock()
		s.send(false)
		if s.stopped(ctx) != nil {
			break
		}
	}
	return s.stopped(ctx)
}
