package components

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/remanence/engine/scan"
	"github.com/mehran9675/remanence/ui/theme"
)

// CellColor is the color a sector map cell is drawn in.
func CellColor(c scan.Cell) string {
	switch c {
	case scan.CellRead:
		return giode.Mix("theme.text", "theme.surface", 0.30)
	case scan.CellMetadata:
		return "theme.info"
	case scan.CellFound:
		return "theme.primary"
	case scan.CellBad:
		return "theme.danger"
	}
	return giode.Mix("theme.text", "theme.surface", 0.05)
}

// SectorMap draws the whole drive as a grid of cells, cols wide, each
// colored by the most interesting thing found in its stretch of sectors.
func SectorMap(cells []scan.Cell, cols int) giode.Element {
	return giode.Raw(func(gtx layout.Context) layout.Dimensions {
		n := scan.MapCells
		rows := (n + cols - 1) / cols
		gap := gtx.Dp(unit.Dp(2))
		w := gtx.Constraints.Max.X
		cell := max(2, (w-gap*(cols-1))/cols)
		h := rows*(cell+gap) - gap
		var palette [5]color.NRGBA
		for i := range palette {
			palette[i] = rgba(CellColor(scan.Cell(i)))
		}
		r := gtx.Dp(1)
		for i := range n {
			c := scan.CellUnread
			if i < len(cells) {
				c = cells[i]
			}
			x, y := (i%cols)*(cell+gap), (i/cols)*(cell+gap)
			rect := image.Rect(x, y, x+cell, y+cell)
			st := clip.UniformRRect(rect, r).Push(gtx.Ops)
			paint.ColorOp{Color: palette[c]}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			st.Pop()
		}
		return layout.Dimensions{Size: image.Pt(w, h)}
	})
}

// SectorLegend explains the map's colors.
func SectorLegend() giode.Element {
	item := func(c scan.Cell, label string) giode.Element {
		return giode.Row(giode.Styles{Gap: 6, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
			giode.Box(giode.Styles{Width: 9, Height: 9, BorderRadius: 2, Background: CellColor(c)}),
			giode.Text(label, theme.Caption()),
		)
	}
	return giode.Row(giode.Styles{Gap: theme.S4, Align: giode.AlignCenter},
		item(scan.CellRead, "Read"),
		item(scan.CellMetadata, "File system"),
		item(scan.CellFound, "Found by signature"),
		item(scan.CellBad, "Unreadable"),
		item(scan.CellUnread, "Not yet read"),
	)
}
