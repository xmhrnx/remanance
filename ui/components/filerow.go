package components

import (
	"time"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/styles/properties"
	"github.com/mehran9675/remanence/engine/fsys"
	"github.com/mehran9675/remanence/engine/recovery"
	"github.com/mehran9675/remanence/ui/theme"
)

// The table's column widths, in dp; the name takes the rest.
const (
	ColSize     = 84
	ColModified = 116
	ColState    = 100
	ColHealth   = 84
)

// RowHeight is the height of a table row, in dp.
const RowHeight = 40

// FileRow draws one row of the results table. compact drops the columns
// for a phone, folding the size and state under the name.
func FileRow(e *fsys.Entry, checked, selected, compact bool, s giode.PressState) giode.Element {
	bg := ""
	switch {
	case selected:
		bg = theme.Tint()
	case s.Hovered:
		bg = theme.Raised()
	}
	nameColor := "theme.text"
	if e.State == fsys.Existing && !e.IsDir {
		nameColor = giode.Mix("theme.text", "theme.surface", 0.72)
	}
	name := giode.Text(e.Name, theme.Body().With(giode.Styles{Color: nameColor, TextOverflow: giode.TextOverflowEllipsis}))
	size := ""
	if !e.IsDir {
		size = recovery.FormatBytes(e.Size)
	}
	row := giode.Styles{Width: -1, Height: -1, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, Gap: theme.S3,
		Padding: giode.Inset{Left: 12, Right: theme.S4}, Background: bg, Cursor: giode.CursorPointer}
	check := giode.Box(giode.Styles{Width: CheckZone - 12, Align: giode.AlignCenter}, CheckMark(checked, s.Hovered))
	if compact {
		sub := size
		if !e.IsDir {
			sub += "  ·  " + StateLabel(e.State)
		}
		return giode.Row(row,
			check,
			FileIcon(e, 18),
			giode.Stack(giode.Styles{FlexGrow: 1, Gap: 1}, name, giode.Text(sub, theme.Caption())),
		)
	}
	state, health := giode.Element(giode.Box(giode.Styles{})), giode.Element(giode.Box(giode.Styles{}))
	if !e.IsDir {
		state = StatePill(e.State)
		if e.State != fsys.Existing {
			health = HealthBar(e.Health)
		}
	} else if e.HasDeleted() {
		state = Tag("Has deleted", "theme.muted")
	}
	return giode.Row(row,
		check,
		FileIcon(e, 18),
		giode.Box(giode.Styles{FlexGrow: 1}, name),
		giode.Box(giode.Styles{Width: ColSize, Justify: giode.JustifyEnd}, giode.Text(size, theme.Code())),
		giode.Box(giode.Styles{Width: ColModified}, giode.Text(FormatTime(e.Modified), theme.Secondary().With(giode.Styles{FontSize: 12.5}))),
		giode.Box(giode.Styles{Width: ColState}, state),
		giode.Box(giode.Styles{Width: ColHealth}, health),
	)
}

// FileHeader is the table's column header.
func FileHeader(check giode.Element) giode.Element {
	h := func(label string, w int, end bool) giode.Element {
		st := giode.Styles{}
		if w > 0 {
			st.Width = properties.Width(w)
		}
		if end {
			st.Justify = giode.JustifyEnd
		}
		return giode.Box(st, giode.Text(label, theme.Overline()))
	}
	return giode.Row(giode.Styles{Width: -1, Height: 34, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, Gap: theme.S3,
		Padding: giode.Inset{Left: 12, Right: theme.S4}},
		giode.Box(giode.Styles{Width: CheckZone - 12, Align: giode.AlignCenter}, check),
		giode.Box(giode.Styles{Width: 18}),
		giode.Box(giode.Styles{FlexGrow: 1}, giode.Text("Name", theme.Overline())),
		h("Size", ColSize, true),
		h("Modified", ColModified, false),
		h("State", ColState, false),
		h("Recovery", ColHealth, false),
	)
}

// FormatTime shows a file time the short way, or a dash when unknown.
func FormatTime(t time.Time) string {
	if t.IsZero() || t.Year() < 1981 {
		return "—"
	}
	if t.Year() == time.Now().Year() {
		return t.Local().Format("Jan 2, 15:04")
	}
	return t.Local().Format("Jan 2, 2006")
}
