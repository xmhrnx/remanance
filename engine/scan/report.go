package scan

import (
	"sort"
	"time"
)

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
		Current:  -1,
	}
	if s.phase == PhaseSectors {
		p.Current = s.sm.cell(s.opts.Start + p.Position)
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
