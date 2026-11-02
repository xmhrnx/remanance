// Package components holds the pieces DataDoc's screens are built
// from, one per file.
package components

import (
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/mehran9675/giode/styles/properties"
)

// rgba resolves a color or theme token for drawing.
func rgba(c string) color.NRGBA { return properties.ResolveColor(c) }

// arc is the outline of a circle's arc from a0 to a1 radians, as true
// curves: polylines of many short segments leave seams in Gio's
// renderer.
func arc(ops *op.Ops, c f32.Point, r, a0, a1 float32) clip.PathSpec {
	var p clip.Path
	p.Begin(ops)
	p.MoveTo(f32.Pt(c.X+r*float32(math.Cos(float64(a0))), c.Y+r*float32(math.Sin(float64(a0)))))
	p.ArcTo(c, c, a1-a0)
	return p.End()
}

// dot fills a circle.
func dot(ops *op.Ops, c f32.Point, r float32, col color.NRGBA) {
	defer clip.Ellipse{Min: f32.Pt(c.X-r, c.Y-r).Round(), Max: f32.Pt(c.X+r, c.Y+r).Round()}.Op(ops).Push(ops).Pop()
	paint.ColorOp{Color: col}.Add(ops)
	paint.PaintOp{}.Add(ops)
}

func withAlpha(c color.NRGBA, a float32) color.NRGBA {
	c.A = uint8(float32(c.A) * a)
	return c
}

// paintStroke strokes path width wide in col.
func paintStroke(ops *op.Ops, path clip.PathSpec, width float32, col color.NRGBA) {
	defer clip.Stroke{Path: path, Width: width}.Op().Push(ops).Pop()
	paint.ColorOp{Color: col}.Add(ops)
	paint.PaintOp{}.Add(ops)
}

// lerp mixes a toward b by t.
func lerp(a, b color.NRGBA, t float32) color.NRGBA {
	m := func(x, y uint8) uint8 { return uint8(float32(x) + (float32(y)-float32(x))*t) }
	return color.NRGBA{R: m(a.R, b.R), G: m(a.G, b.G), B: m(a.B, b.B), A: m(a.A, b.A)}
}
