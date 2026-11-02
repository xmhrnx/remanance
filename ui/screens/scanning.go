package screens

import (
	"fmt"
	"time"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/scroll"
	"github.com/mehran9675/giode/styles/properties"
	"github.com/xmhrnx/remanance/engine/disk"
	"github.com/xmhrnx/remanance/engine/recovery"
	"github.com/xmhrnx/remanance/engine/scan"
	"github.com/xmhrnx/remanance/locales"
	"github.com/xmhrnx/remanance/ui/components"
	"github.com/xmhrnx/remanance/ui/model"
	"github.com/xmhrnx/remanance/ui/theme"
)

// Scanning shows a scan in progress: the ring, what's been found, and
// the sector map filling in.
type Scanning struct {
	m    *model.Model
	ring components.ProgressRing
	stop *giode.ButtonElement
	view *scroll.Scroll
}

// NewScanning builds the scan screen.
func NewScanning(m *model.Model) *Scanning {
	s := &Scanning{m: m}
	s.stop = components.SecondaryButton(locales.T("scanning.stop_and_show_results"), "stop").OnClick(m.CancelScan)
	s.view = giode.Scroll(s.content, giode.Styles{ScrollBar: giode.ScrollBar{AutoHide: true}})
	return s
}

// View lays out the screen.
func (s *Scanning) View() giode.Element { return s.view }

func (s *Scanning) content() giode.Element {
	p := s.m.Progress
	compact := giode.Compact()
	ringSize := 280
	if compact {
		ringSize = 220
	}
	pct := fmt.Sprintf("%d%%", int(p.Fraction*100))
	center := giode.Stack(giode.Styles{Align: giode.AlignCenter, Gap: 4},
		giode.Text(pct, theme.Figure(46).With(giode.Styles{FontWeight: giode.FontWeightBold})),
		giode.Box(giode.Styles{MaxWidth: properties.MaxWidth(ringSize - 70)}, giode.Text(label(string(p.Phase)), theme.Secondary().With(giode.Styles{TextAlign: giode.TextAlignCenter}))),
	)

	name := s.m.SourceName()
	mode := locales.T("scanning.quick_scan")
	if s.m.Mode == scan.Deep {
		mode = locales.T("scanning.deep_scan")
	}
	var speed string
	if p.Phase == scan.PhaseSectors && p.Speed > 0 {
		speed = locales.T("scanning.speed", "read", recovery.FormatBytes(p.Position), "size", recovery.FormatBytes(p.Total),
			"speed", recovery.FormatBytes(int64(p.Speed)), "left", friendly(p.Remaining))
	}

	files := components.Stat("file", "theme.text", fmt.Sprint(p.Files), locales.T("scanning.files_in_file_systems"))
	deleted := components.Stat("ghost", "theme.primary", fmt.Sprint(p.Deleted), locales.T("scanning.deleted"))
	carved := components.Stat("sparkle", "theme.info", fmt.Sprint(p.Carved), locales.T("scanning.found_by_signature"))
	vols := components.Stat("layers", "theme.accent", fmt.Sprint(p.Partitions), locales.T("scanning.partitions"))
	row := func(kids ...giode.Element) giode.Element {
		return giode.Row(giode.Styles{Width: -1, Gap: theme.S3, Wrap: giode.WrapNoWrap}, kids...)
	}
	stats := row(files, deleted, carved, vols)
	if compact {
		stats = giode.Stack(giode.Styles{Width: -1, Gap: theme.S3}, row(files, deleted), row(carved, vols))
	}
	reading := locales.T("scanning.reading_size", "size", recovery.FormatBytes(s.m.Source.Size))
	if s.m.Source.Kind == disk.KindFiles {
		// Files: their size is known only once they're listed.
		reading = locales.T("scanning.reading_only_nothing_is_written")
	}
	parts := []giode.Element{
		giode.Stack(giode.Styles{Gap: 4, Align: giode.AlignCenter},
			giode.Text(locales.T("scanning.mode_of", "mode", mode, "name", name), theme.Title().With(giode.Styles{TextAlign: giode.TextAlignCenter})),
			giode.Text(reading, theme.Secondary()),
		),
		s.ring.View(p.Fraction, ringSize, s.m.Scanning, center),
		giode.Text(speed, theme.Code()),
		stats,
	}
	if s.m.Mode == scan.Deep {
		cols := 80
		if compact {
			cols = 40
		}
		current := -1
		if s.m.Scanning {
			current = p.Current
		}
		parts = append(parts, components.Card(giode.Styles{Width: -1, Gap: theme.S4},
			giode.Row(giode.Styles{Width: -1, Align: giode.AlignCenter, Justify: giode.JustifySpaceBetween},
				giode.Text(locales.T("scanning.sector_map"), theme.Heading()),
				giode.Text(badLine(p.BadBytes), theme.Caption().With(giode.Styles{Color: "theme.danger"})),
			),
			components.SectorMap(p.Map, cols, current),
			components.SectorLegend(),
		))
	}
	parts = append(parts, s.stop)
	pad := theme.S8
	if compact {
		pad = theme.S4
	}
	return giode.Box(giode.Styles{Width: -1, Justify: giode.JustifyCenter},
		giode.Stack(giode.Styles{Width: -1, MaxWidth: 920, Padding: giode.UniformInset(pad), Gap: theme.S6, Align: giode.AlignCenter}, parts...))
}

func badLine(n int64) string {
	if n == 0 {
		return ""
	}
	return locales.T("scanning.unreadable", "size", recovery.FormatBytes(n))
}

// friendly rounds a duration the way people say it.
func friendly(d time.Duration) string {
	switch {
	case d <= 0:
		return locales.T("scanning.a_moment")
	case d < time.Minute:
		return locales.T("time.seconds", "s", int(d.Seconds()))
	case d < time.Hour:
		return locales.T("time.minutes", "m", int(d.Minutes()+0.5))
	}
	return locales.T("time.hours", "h", int(d.Hours()), "m", int(d.Minutes())%60)
}
