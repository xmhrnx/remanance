package components

import (
	"image"
	"image/color"
	"math"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/mehran9675/giode"
	"github.com/xmhrnx/remanance/engine/scan"
	"github.com/xmhrnx/remanance/ui/theme"
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
// The cell at current, the one being read, blinks; -1 marks none.
func SectorMap(cells []scan.Cell, cols, current int) giode.Element {
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
		if current >= 0 && current < n {
			// Once a second, fading between the cell's own color and a
			// bright amber a little larger than the cell.
			t := float64(gtx.Now.UnixMilli()%1000) / 1000
			on := float32(0.5 + 0.5*math.Cos(2*math.Pi*t))
			x, y := (current%cols)*(cell+gap), (current/cols)*(cell+gap)
			grow := gtx.Dp(1)
			rect := image.Rect(x-grow, y-grow, x+cell+grow, y+cell+grow)
			st := clip.UniformRRect(rect, r+grow).Push(gtx.Ops)
			paint.ColorOp{Color: withAlpha(rgba("#FFD27A"), on)}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			st.Pop()
			gtx.Execute(op.InvalidateCmd{})
		}
		return layout.Dimensions{Size: image.Pt(w, h)}
	})
}

// SectorLegend explains the map's colors.
func SectorLegend() giode.Element {
	swatch := func(color, label string) giode.Element {
		return giode.Row(giode.Styles{Gap: 6, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
			giode.Box(giode.Styles{Width: 9, Height: 9, BorderRadius: 2, Background: color}),
			giode.Text(label, theme.Caption()),
		)
	}
	item := func(c scan.Cell, label string) giode.Element { return swatch(CellColor(c), label) }
	return giode.Row(giode.Styles{Gap: theme.S4, Align: giode.AlignCenter},
		swatch("#FFD27A", "Reading now"),
		item(scan.CellRead, "Read"),
		item(scan.CellMetadata, "File system"),
		item(scan.CellFound, "Found by signature"),
		item(scan.CellBad, "Unreadable"),
		item(scan.CellUnread, "Not yet read"),
	)
}
