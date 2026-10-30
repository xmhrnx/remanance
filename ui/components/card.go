package components

import (
	"github.com/mehran9675/giode"
	"github.com/mehran9675/remanence/ui/theme"
)

// CardStyles is the panel every card sits on: the surface, a hairline
// border and a soft shadow. Extend it with With.
func CardStyles() giode.Styles {
	return giode.Styles{
		FlexDirection: giode.FlexDirectionColumn,
		Background:    "theme.surface",
		BorderWidth:   1,
		BorderColor:   "theme.border",
		BorderRadius:  theme.RadiusLarge,
		Padding:       giode.UniformInset(theme.S5),
		Gap:           theme.S3,
		BoxShadow:     giode.BoxShadow{Y: 8, Blur: 24, Color: "#00000038"},
	}
}

// Card is a panel holding children in a column.
func Card(st giode.Styles, children ...giode.Element) giode.Element {
	return giode.Box(CardStyles().With(st), children...)
}
