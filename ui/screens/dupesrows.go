package screens

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mehran9675/giode"
	"github.com/xmhrnx/remanance/engine/dupes"
	"github.com/xmhrnx/remanance/engine/recovery"
	"github.com/xmhrnx/remanance/locales"
	"github.com/xmhrnx/remanance/ui/components"
	"github.com/xmhrnx/remanance/ui/icons"
	"github.com/xmhrnx/remanance/ui/theme"
)

// visible flattens the groups the filter shows into rows, again only
// when they change.
func (d *Dupes) visible() []dupeRow {
	gs := d.m.Dupes.Groups
	show := d.show.Selected()
	if len(gs) == len(d.rowsFor) && (len(gs) == 0 || &gs[0] == &d.rowsFor[0]) && show == d.rowsOf && d.rows != nil && sameSizes(gs, d.rowsFor) {
		return d.rows
	}
	d.rows = d.rows[:0]
	for gi, g := range gs {
		if show == 1 && g.Kind != dupes.Exact || show == 2 && g.Kind == dupes.Exact {
			continue
		}
		d.rows = append(d.rows, dupeRow{gi, -1})
		for fi := range g.Files {
			d.rows = append(d.rows, dupeRow{gi, fi})
		}
	}
	d.rowsFor, d.rowsOf = gs, show
	return d.rows
}

func sameSizes(a, b []dupes.Group) bool {
	for i := range a {
		if len(a[i].Files) != len(b[i].Files) {
			return false
		}
	}
	return true
}

func (d *Dupes) row(i int) giode.Element {
	rows := d.visible()
	if i >= len(rows) {
		return giode.Box(giode.Styles{})
	}
	r := rows[i]
	g := d.m.Dupes.Groups[r.group]
	if r.file < 0 {
		return d.header(g)
	}
	f := g.Files[r.file]
	p := giode.Keyed("dupe-row", f.Path, func() *giode.PressableElement {
		return giode.Pressable(func(giode.PressState) giode.Element { return nil })
	})
	return p.Content(func(s giode.PressState) giode.Element {
		return d.fileRow(g, f, s)
	}).OnClick(func(giode.ClickEvent) { d.m.ToggleDupe(g, f.Path) }).Label(filepath.Base(f.Path))
}

func (d *Dupes) header(g dupes.Group) giode.Element {
	icon, color := "copy", "theme.info"
	detail := locales.T("dupes.copies_each", "copies", count(len(g.Files), "copy"), "size", recovery.FormatBytes(g.Files[0].Size))
	switch g.Kind {
	case dupes.SimilarPhoto:
		icon, color = "picture", "#E58FB0"
		detail = count(len(g.Files), "photo")
		if g.Distance == 0 {
			detail = locales.T("dupes.same_picture", "photos", detail)
		}
	case dupes.SimilarVideo:
		icon, color = "video", "#A98BF0"
		detail = locales.T("dupes.videos_long", "videos", count(len(g.Files), "video"), "length", g.Files[0].Duration.Round(time.Second))
	}
	return giode.Row(giode.Styles{Width: -1, Height: -1, Gap: theme.S3, Align: giode.AlignEnd, Wrap: giode.WrapNoWrap,
		Padding: giode.Inset{Left: theme.S2, Right: theme.S3, Bottom: theme.S2}},
		icons.Icon(icon, 16, color),
		giode.Text(label(g.Kind.Label()), theme.Body().With(giode.Styles{FontSize: 13, FontWeight: giode.FontWeightSemiBold})),
		giode.Box(giode.Styles{FlexGrow: 1}, giode.Text(detail, theme.Caption())),
		giode.Text(locales.T("dupes.extra", "size", recovery.FormatBytes(g.Wasted())), theme.Caption().With(giode.Styles{Color: "theme.primary"})),
	)
}

func (d *Dupes) fileRow(g dupes.Group, f dupes.File, s giode.PressState) giode.Element {
	checked := d.m.Dupes.Remove[f.Path]
	bg := "theme.surface"
	switch {
	case checked:
		bg = giode.Mix("theme.danger", "theme.surface", 0.07)
	case s.Hovered:
		bg = theme.Raised()
	}
	var thumb giode.Element
	if g.Kind != dupes.SimilarVideo && isPicture(f.Path) {
		thumb = d.thumbs.View(f.Path, 48)
	} else {
		thumb = giode.Box(giode.Styles{Width: 48, Height: 48, BorderRadius: theme.RadiusSmall, Background: theme.Sunken(),
			Align: giode.AlignCenter, Justify: giode.JustifyCenter}, components.FileIconNamed(f.Path, 22))
	}
	var facts []string
	facts = append(facts, recovery.FormatBytes(f.Size))
	if f.Width > 0 {
		facts = append(facts, fmt.Sprintf("%d×%d", f.Width, f.Height))
	}
	facts = append(facts, f.Modified.Format("Jan 2, 2006"))
	tags := []giode.Element{}
	if f.Keep {
		tags = append(tags, components.Tag(locales.T("dupes.suggested_to_keep"), "theme.success"))
	}
	if checked {
		tags = append(tags, components.Tag(locales.T("dupes.will_be_removed"), "theme.danger"))
	}
	return giode.Row(giode.Styles{Width: -1, Height: dupeRowHeight - 4, Gap: theme.S3, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap,
		Padding: giode.SymmetricInset(theme.S3, 0), Background: bg, BorderRadius: theme.RadiusSmall, BorderWidth: 1, BorderColor: theme.Hairline()},
		components.CheckMark(checked, s.Hovered),
		thumb,
		giode.Stack(giode.Styles{FlexGrow: 1, Gap: 3},
			giode.Text(filepath.Base(f.Path), theme.Body().With(giode.Styles{FontSize: 13, FontWeight: giode.FontWeightMedium, MaxLines: 1, TextOverflow: giode.TextOverflowEllipsis})),
			giode.Text(filepath.Dir(f.Path), theme.Caption().With(giode.Styles{FontSize: 11.5, MaxLines: 1, TextOverflow: giode.TextOverflowEllipsis})),
		),
		giode.Row(giode.Styles{Gap: theme.S2, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap}, tags...),
		giode.Box(giode.Styles{Width: 190}, giode.Text(join(facts), theme.Caption().With(giode.Styles{TextAlign: giode.TextAlignEnd, MaxLines: 1}))),
	)
}

func join(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += locales.T("common.separator")
		}
		out += p
	}
	return out
}

func isPicture(p string) bool {
	switch filepath.Ext(p) {
	case ".jpg", ".jpeg", ".png", ".gif", ".JPG", ".JPEG", ".PNG", ".GIF", ".bmp", ".webp":
		return true
	}
	return false
}
