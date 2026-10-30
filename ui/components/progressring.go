package components

import (
	"image"
	"math"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/styles/properties"
)

// ProgressRing is the large ring on the scan screen: a faint track and an
// amber arc that eases toward the scan's progress, with a glowing head
// like the trace in the icon. Create it once; it remembers where the arc
// was drawn last so it can animate.
type ProgressRing struct {
	shown float32
	last  time.Time
	spin  float32
}

// View draws the ring size dp across with center inside it. Active
// keeps a slow sweep going while the fraction is still unknown.
func (r *ProgressRing) View(fraction float64, size int, active bool, center giode.Element) giode.Element {
	target := float32(math.Max(0, math.Min(1, fraction)))
	ring := giode.Raw(func(gtx layout.Context) layout.Dimensions {
		px := gtx.Dp(unit.Dp(size))
		dt := float32(0.016)
		if !r.last.IsZero() {
			dt = min(0.1, float32(gtx.Now.Sub(r.last).Seconds()))
		}
		r.last = gtx.Now
		// Ease toward the target; jumps backwards (a new scan) are instant.
		if target < r.shown {
			r.shown = target
		} else {
			r.shown += (target - r.shown) * min(1, dt*6)
		}
		if active {
			r.spin = float32(math.Mod(float64(r.spin+dt*0.9), 2*math.Pi))
		}

		ops := gtx.Ops
		stroke := float32(gtx.Dp(9))
		c := f32.Pt(float32(px)/2, float32(px)/2)
		rad := float32(px)/2 - stroke
		track := rgba(giode.Mix("theme.text", "theme.background", 0.07))
		amber, deep := rgba("theme.primary"), rgba("theme.secondary")

		// Groove lines inside, like the platter's.
		for i, k := range []float32{0.80, 0.70, 0.60} {
			a := float32(0.05 - 0.012*float32(i))
			paintStroke(ops, arc(ops, c, rad*k, 0, 2*math.Pi), float32(gtx.Dp(1)), withAlpha(rgba("theme.text"), a))
		}
		paintStroke(ops, arc(ops, c, rad, 0, 2*math.Pi), stroke, track)

		start := float32(-math.Pi / 2)
		if r.shown > 0.002 {
			end := start + 2*math.Pi*r.shown
			func() {
				defer clip.Stroke{Path: arc(ops, c, rad, start, end), Width: stroke}.Op().Push(ops).Pop()
				paint.LinearGradientOp{
					Stop1: f32.Pt(c.X-rad, c.Y+rad), Color1: deep,
					Stop2: f32.Pt(c.X+rad, c.Y-rad), Color2: amber,
				}.Add(ops)
				paint.PaintOp{}.Add(ops)
			}()
			// The round start cap, in the gradient's color at the top: 3/4 of
			// the way from its bottom-left stop to its top-right one.
			dot(ops, f32.Pt(c.X, c.Y-rad), stroke/2, lerp(deep, amber, 0.75))
			head := f32.Pt(c.X+rad*float32(math.Cos(float64(end))), c.Y+rad*float32(math.Sin(float64(end))))
			dot(ops, head, stroke*1.25, withAlpha(amber, 0.22))
			dot(ops, head, stroke*0.72, rgba("#FFE6BC"))
			dot(ops, head, stroke*0.36, rgba("#FFFFFF"))
		}
		if active {
			// A faint comet sweeping the track while the scan runs.
			a := start + r.spin
			paintStroke(ops, arc(ops, c, rad, a-0.6, a), stroke*0.35, withAlpha(rgba("theme.text"), 0.16))
		}
		if active || math.Abs(float64(target-r.shown)) > 0.001 {
			gtx.Execute(op.InvalidateCmd{})
		}
		return layout.Dimensions{Size: image.Pt(px, px)}
	})
	return giode.Box(giode.Styles{
		Width: properties.Width(size), Height: properties.Height(size),
		Align: giode.AlignCenter, Justify: giode.JustifyCenter, FlexDirection: giode.FlexDirectionColumn,
		Position: giode.PositionRelative,
	},
		center,
		giode.Box(giode.Styles{Position: giode.PositionAbsolute, Left: giode.Flush, Top: giode.Flush}, ring),
	)
}
