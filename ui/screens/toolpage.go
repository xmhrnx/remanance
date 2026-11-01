package screens

import (
	"github.com/mehran9675/giode"
	"github.com/xmhrnx/remanance/ui/icons"
	"github.com/xmhrnx/remanance/ui/theme"
)

// toolHeader opens a tool's first page: its icon, what it does and how.
func toolHeader(icon, title, sub string) giode.Element {
	return giode.Row(giode.Styles{Width: -1, Gap: theme.S4, Align: giode.AlignStart, Wrap: giode.WrapNoWrap},
		giode.Box(giode.Styles{Width: 52, Height: 52, BorderRadius: 14, Align: giode.AlignCenter, Justify: giode.JustifyCenter,
			Background: giode.Mix("theme.primary", "theme.surface", 0.14)}, icons.Icon(icon, 26, "theme.primary")),
		giode.Stack(giode.Styles{FlexGrow: 1, Gap: theme.S2},
			giode.Text(title, theme.Hero()),
			giode.Box(giode.Styles{MaxWidth: 640}, giode.Text(sub, theme.Secondary().With(giode.Styles{FontSize: 14, LineHeight: 21}))),
		),
	)
}

// page centers a tool's page and pads it, less on a phone.
func page(compact bool, kids ...giode.Element) giode.Element {
	pad := theme.S8
	if compact {
		pad = theme.S4
	}
	return giode.Box(giode.Styles{Width: -1, Justify: giode.JustifyCenter},
		giode.Stack(giode.Styles{Width: -1, MaxWidth: 1080, Padding: giode.UniformInset(pad), Gap: theme.S6}, kids...))
}

// section is a titled part of a page.
func section(title string, body ...giode.Element) giode.Element {
	return giode.Stack(giode.Styles{Width: -1, Gap: theme.S3},
		append([]giode.Element{giode.Text(title, theme.Overline().With(giode.Styles{FontSize: 11}))}, body...)...)
}
