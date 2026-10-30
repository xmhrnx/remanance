package components

import (
	"github.com/mehran9675/giode"
	"github.com/mehran9675/remanence/ui/icons"
	"github.com/mehran9675/remanence/ui/theme"
)

// EmptyState fills a pane that has nothing to show, saying why and what
// to do about it.
func EmptyState(icon, title, message string, action giode.Element) giode.Element {
	kids := []giode.Element{
		giode.Box(giode.Styles{Width: 56, Height: 56, BorderRadius: 28, Align: giode.AlignCenter, Justify: giode.JustifyCenter,
			Background: giode.Mix("theme.primary", "theme.surface", 0.12)},
			icons.Icon(icon, 26, "theme.primary")),
		giode.Text(title, theme.Heading().With(giode.Styles{TextAlign: giode.TextAlignCenter})),
		giode.Box(giode.Styles{MaxWidth: 380}, giode.Text(message, theme.Secondary().With(giode.Styles{TextAlign: giode.TextAlignCenter}))),
	}
	if action != nil {
		kids = append(kids, giode.Box(giode.Styles{Padding: giode.Inset{Top: theme.S2}}, action))
	}
	return giode.Stack(giode.Styles{Width: -1, Height: -1, Align: giode.AlignCenter, Justify: giode.JustifyCenter, Gap: theme.S3,
		Padding: giode.UniformInset(theme.S6)}, kids...)
}
