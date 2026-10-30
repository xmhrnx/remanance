package components

import (
	"github.com/mehran9675/giode"
	"github.com/mehran9675/remanence/ui/icons"
	"github.com/mehran9675/remanence/ui/theme"
)

// Banner is a message on a wash of color with an optional action after
// it: a warning above the drive list, a note on a file.
func Banner(icon, color, message string, action giode.Element) giode.Element {
	kids := []giode.Element{
		giode.Box(giode.Styles{Padding: giode.Inset{Top: 1}}, icons.Icon(icon, 16, color)),
		giode.Box(giode.Styles{FlexGrow: 1}, giode.Text(message, theme.Body().With(giode.Styles{FontSize: 13}))),
	}
	if action != nil {
		kids = append(kids, action)
	}
	return giode.Row(giode.Styles{
		Width: -1, Gap: theme.S3, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap,
		Padding: giode.SymmetricInset(theme.S4, theme.S3), BorderRadius: theme.Radius,
		Background: giode.Mix(color, "theme.surface", 0.10), BorderWidth: 1, BorderColor: giode.Mix(color, "theme.surface", 0.28),
	}, kids...)
}
