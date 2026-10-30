package screens

import (
	"fmt"
	"time"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/scroll"
	"github.com/mehran9675/giode/styles/properties"
	"github.com/mehran9675/remanence/engine/recovery"
	"github.com/mehran9675/remanence/engine/scan"
	"github.com/mehran9675/remanence/ui/components"
	"github.com/mehran9675/remanence/ui/model"
	"github.com/mehran9675/remanence/ui/theme"
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
	s.stop = components.SecondaryButton("Stop and show results", "stop").OnClick(m.CancelScan)
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
		giode.Box(giode.Styles{MaxWidth: properties.MaxWidth(ringSize - 70)}, giode.Text(string(p.Phase), theme.Secondary().With(giode.Styles{TextAlign: giode.TextAlignCenter}))),
	)

	name := s.m.Source.Model
	if name == "" {
		name = s.m.Source.Name
	}
	mode := "Quick scan"
	if s.m.Mode == scan.Deep {
		mode = "Deep scan"
	}
	var speed string
	if p.Phase == scan.PhaseSectors && p.Speed > 0 {
		speed = fmt.Sprintf("%s of %s  ·  %s/s  ·  about %s left",
			recovery.FormatBytes(p.Position), recovery.FormatBytes(p.Total), recovery.FormatBytes(int64(p.Speed)), friendly(p.Remaining))
	}

	files := components.Stat("file", "theme.text", fmt.Sprint(p.Files), "Files in file systems")
	deleted := components.Stat("ghost", "theme.primary", fmt.Sprint(p.Deleted), "Deleted")
	carved := components.Stat("sparkle", "theme.info", fmt.Sprint(p.Carved), "Found by signature")
	vols := components.Stat("layers", "theme.accent", fmt.Sprint(p.Partitions), "Partitions")
	row := func(kids ...giode.Element) giode.Element {
		return giode.Row(giode.Styles{Width: -1, Gap: theme.S3, Wrap: giode.WrapNoWrap}, kids...)
	}
	stats := row(files, deleted, carved, vols)
	if compact {
		stats = giode.Stack(giode.Styles{Width: -1, Gap: theme.S3}, row(files, deleted), row(carved, vols))
	}
	parts := []giode.Element{
		giode.Stack(giode.Styles{Gap: 4, Align: giode.AlignCenter},
			giode.Text(mode+" of "+name, theme.Title().With(giode.Styles{TextAlign: giode.TextAlignCenter})),
			giode.Text(recovery.FormatBytes(s.m.Source.Size)+" · reading only, nothing is written", theme.Secondary()),
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
		parts = append(parts, components.Card(giode.Styles{Width: -1, Gap: theme.S4},
			giode.Row(giode.Styles{Width: -1, Align: giode.AlignCenter, Justify: giode.JustifySpaceBetween},
				giode.Text("Sector map", theme.Heading()),
				giode.Text(badLine(p.BadBytes), theme.Caption().With(giode.Styles{Color: "theme.danger"})),
			),
			components.SectorMap(p.Map, cols),
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
	return recovery.FormatBytes(n) + " unreadable, skipped"
}

// friendly rounds a duration the way people say it.
func friendly(d time.Duration) string {
	switch {
	case d <= 0:
		return "a moment"
	case d < time.Minute:
		return fmt.Sprintf("%d s", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%d min", int(d.Minutes()+0.5))
	}
	return fmt.Sprintf("%d h %d min", int(d.Hours()), int(d.Minutes())%60)
}
