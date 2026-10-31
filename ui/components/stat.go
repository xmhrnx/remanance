package components

import (
	"github.com/mehran9675/giode"
	"github.com/xmhrnx/remanance/ui/icons"
	"github.com/xmhrnx/remanance/ui/theme"
)

// statHeight fits the figure and two lines of its label.
const statHeight = 84

// Stat is one figure on the scan screen: an icon in a tinted well, the
// number and what it counts. Every stat is statHeight tall, so they
// line up when one label wraps onto a second line and another doesn't.
func Stat(icon, color, value, label string) giode.Element {
	return giode.Row(giode.Styles{
		Gap: theme.S3, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap,
		Padding: giode.SymmetricInset(theme.S4, theme.S3), FlexGrow: 1, MinWidth: 120, Height: statHeight,
		Background: "theme.surface", BorderRadius: theme.Radius, BorderWidth: 1, BorderColor: "theme.border",
	},
		giode.Box(giode.Styles{Width: 36, Height: 36, BorderRadius: theme.Radius, Align: giode.AlignCenter, Justify: giode.JustifyCenter,
			Background: giode.Mix(color, "theme.surface", 0.14)},
			icons.Icon(icon, 18, color)),
		giode.Stack(giode.Styles{Gap: 2},
			giode.Text(value, theme.Figure(20)),
			giode.Text(label, theme.Caption()),
		),
	)
}
