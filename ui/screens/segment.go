package screens

import (
	"github.com/mehran9675/giode"
	"github.com/mehran9675/remanence/ui/theme"
)

// segmentStyles is the look of every segmented control: a dark track and
// a raised segment for the choice.
func segmentStyles() giode.Styles {
	return giode.Styles{
		Width: -1, FontSize: 13, FontFamily: theme.Sans,
		Background:  giode.Mix("theme.text", "theme.background", 0.07),
		BorderColor: giode.Mix("theme.text", "theme.surface", 0.12),
		Color:       "theme.text",
	}
}
