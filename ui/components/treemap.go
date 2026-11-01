package components

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/mehran9675/giode"
	"github.com/xmhrnx/remanance/engine/recovery"
	"github.com/xmhrnx/remanance/engine/space"
	"github.com/xmhrnx/remanance/ui/theme"
)

// maxTiles is the most tiles drawn for one folder; the rest are too
// small to point at, and are drawn as one.
const maxTiles = 160

// Treemap draws a folder as a squarified treemap: a tile for each file
// and folder in it, sized by the space it takes, each folder's own
// contents drawn inside its tile a shade apart. A click selects a
// tile; a double click, or a click on a selected folder, opens it.
type Treemap struct {
	open, sel func(*space.Node)
}

// NewTreemap returns a treemap that calls open to show a folder and
// selected when a tile is picked.
func NewTreemap(open, selected func(*space.Node)) *Treemap {
	return &Treemap{open: open, sel: selected}
}

// View draws cur, with selected outlined.
func (t *Treemap) View(cur, selected *space.Node) giode.Element {
	return giode.Raw(func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		if cur == nil || size.X <= 0 || size.Y <= 0 {
			return layout.Dimensions{Size: size}
		}
		kids := cur.Children
		if len(kids) > maxTiles {
			kids = kids[:maxTiles]
		}
		gap := gtx.Dp(2)
		tiles := space.Layout(kids, space.Rect{W: float64(size.X), H: float64(size.Y)})
		for i, tl := range tiles {
			r := image.Rect(int(tl.X), int(tl.Y), int(tl.X+tl.W), int(tl.Y+tl.H))
			r.Max = r.Max.Sub(image.Pt(gap, gap))
			if r.Dx() < 2 || r.Dy() < 2 {
				continue
			}
			n, idx := tl.Node, i
			p := giode.Keyed("tile", n, func() *giode.PressableElement {
				return giode.Pressable(func(giode.PressState) giode.Element { return nil })
			})
			p.Content(func(s giode.PressState) giode.Element {
				return t.tile(n, idx, n == selected, s)
			}).OnClick(func(e giode.ClickEvent) {
				if n.IsDir && (e.Clicks == 2 || n == selected) {
					t.open(n)
					return
				}
				t.sel(n)
			}).Label(n.Name + ", " + recovery.FormatBytes(n.Size))
			st := op.Offset(r.Min).Push(gtx.Ops)
			g := gtx
			g.Constraints = layout.Exact(r.Size())
			p.Layout(g)
			st.Pop()
		}
		return layout.Dimensions{Size: size}
	})
}

// tile draws one file or folder: its color, its contents a shade apart,
// and its name and size when there's room.
func (t *Treemap) tile(n *space.Node, i int, selected bool, s giode.PressState) giode.Element {
	base := NodeColor(n, i)
	paint := giode.Raw(func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		rad := gtx.Dp(theme.RadiusSmall / 2)
		bg := rgba(giode.Mix(base, "theme.surface", 0.55))
		if s.Hovered {
			bg = rgba(giode.Mix(base, "theme.surface", 0.7))
		}
		fill(gtx.Ops, image.Rectangle{Max: size}, rad, bg)
		if n.IsDir && size.X > 24 && size.Y > 24 {
			// The folder's own contents, inset below its label.
			inset := gtx.Dp(3)
			top := gtx.Dp(20)
			if size.X < 80 || size.Y < 48 {
				top = inset
			}
			area := image.Rect(inset, top, size.X-inset, size.Y-inset)
			kids := n.Children
			if len(kids) > 60 {
				kids = kids[:60]
			}
			sub := space.Layout(kids, space.Rect{X: float64(area.Min.X), Y: float64(area.Min.Y), W: float64(area.Dx()), H: float64(area.Dy())})
			g := gtx.Dp(1)
			for j, tl := range sub {
				r := image.Rect(int(tl.X), int(tl.Y), int(tl.X+tl.W)-g, int(tl.Y+tl.H)-g)
				if r.Dx() < 1 || r.Dy() < 1 {
					continue
				}
				c := base
				if !tl.Node.IsDir {
					c = NodeColor(tl.Node, j)
				}
				mix := 0.78 + 0.06*float64(j%3)
				fill(gtx.Ops, r, gtx.Dp(2), rgba(giode.Mix(c, "theme.surface", mix)))
			}
		}
		if selected {
			defer clip.UniformRRect(image.Rectangle{Max: size}, rad).Push(gtx.Ops).Pop()
			paintStroke(gtx.Ops, clip.UniformRRect(image.Rectangle{Max: size}, rad).Path(gtx.Ops), float32(gtx.Dp(4)), rgba("theme.text"))
		}
		return layout.Dimensions{Size: size}
	})
	kids := []giode.Element{giode.Box(giode.Styles{Position: giode.PositionAbsolute, Width: -1, Height: -1}, paint)}
	kids = append(kids, giode.Stack(giode.Styles{Padding: giode.Inset{Left: 6, Top: 3, Right: 6}, Gap: 0},
		giode.Text(n.Name, giode.Styles{FontSize: 12, FontWeight: giode.FontWeightSemiBold, Color: "theme.text", MaxLines: 1, TextOverflow: giode.TextOverflowEllipsis}),
		giode.Text(recovery.FormatBytes(n.Size), giode.Styles{FontSize: 11, Color: "theme.text/0.7", MaxLines: 1}),
	))
	return giode.Box(giode.Styles{Width: -1, Height: -1, Overflow: giode.OverflowHidden, BorderRadius: theme.RadiusSmall / 2}, kids...)
}

// fill fills a rounded rectangle.
func fill(ops *op.Ops, r image.Rectangle, rad int, c color.NRGBA) {
	defer clip.UniformRRect(r, rad).Push(ops).Pop()
	paint.ColorOp{Color: c}.Add(ops)
	paint.PaintOp{}.Add(ops)
}
