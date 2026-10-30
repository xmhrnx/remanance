package components

import (
	"github.com/mehran9675/giode"
	"github.com/mehran9675/remanence/ui/theme"
)

// Tag is a small rounded label in color, on a faint wash of it.
func Tag(label, color string) giode.Element {
	return giode.Box(giode.Styles{
		Padding: giode.SymmetricInset(7, 2), BorderRadius: theme.RadiusSmall,
		Background: giode.Mix(color, "theme.surface", 0.13),
	}, giode.Text(label, giode.Styles{FontFamily: theme.Sans, FontSize: 11, FontWeight: giode.FontWeightMedium, Color: color}))
}
