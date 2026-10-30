package components

import (
	"time"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/styles/properties"
	"github.com/xmhrnx/remanance/engine/fsys"
	"github.com/xmhrnx/remanance/engine/recovery"
	"github.com/xmhrnx/remanance/ui/icons"
	"github.com/xmhrnx/remanance/ui/model"
	"github.com/xmhrnx/remanance/ui/theme"
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

// FileHeader is the table's column header. Clicking a column's label
// calls onSort with it; the column the table is sorted by shows an
// arrow pointing the way it runs.
func FileHeader(check giode.Element, order model.Order, onSort func(model.Column)) giode.Element {
	h := func(c model.Column, label string, w int, end bool) giode.Element {
		st := giode.Styles{Height: -1}
		if w > 0 {
			st.Width = properties.Width(w)
		} else {
			st.FlexGrow = 1
		}
		p := giode.Keyed("sort", c, func() *giode.PressableElement {
			return giode.Pressable(func(giode.PressState) giode.Element { return nil })
		})
		p.Content(func(s giode.PressState) giode.Element {
			color := "theme.muted"
			if s.Hovered || order.Column == c {
				color = "theme.text"
			}
			kids := []giode.Element{giode.Text(label, theme.Overline().With(giode.Styles{Color: color}))}
			if order.Column == c {
				arrow := "chevron-up"
				if order.Desc {
					arrow = "chevron-down"
				}
				kids = append(kids, icons.Icon(arrow, 13, "theme.primary"))
			}
			row := giode.Styles{Width: -1, Height: -1, Gap: 3, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, Cursor: giode.CursorPointer}
			if end {
				row.Justify = giode.JustifyEnd
			}
			return giode.Row(row, kids...)
		}).OnClick(func(giode.ClickEvent) { onSort(c) }).Label("Sort by " + label)
		return giode.Box(st, p)
	}
	return giode.Row(giode.Styles{Width: -1, Height: 34, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, Gap: theme.S3,
		Padding: giode.Inset{Left: 12, Right: theme.S4}},
		giode.Box(giode.Styles{Width: CheckZone - 12, Align: giode.AlignCenter}, check),
		giode.Box(giode.Styles{Width: 18}),
		h(model.ColName, "Name", 0, false),
		h(model.ColSize, "Size", ColSize, true),
		h(model.ColModified, "Modified", ColModified, false),
		h(model.ColState, "State", ColState, false),
		h(model.ColHealth, "Recovery", ColHealth, false),
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
