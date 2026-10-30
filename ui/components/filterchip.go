package components

import (
	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/chip"
	"github.com/mehran9675/remanence/ui/icons"
)

// FilterChip is a rounded toggle in the filter row: an outline at rest,
// a wash of the accent when on. Create it once; it holds press state.
type FilterChip struct {
	chip *chip.Chip
	icon string
}

// NewFilterChip makes a chip that runs onClick when tapped.
func NewFilterChip(label, icon string, onClick func()) *FilterChip {
	c := giode.Chip(label, giode.Styles{Background: "theme.surface", BorderColor: "theme.border", Color: "theme.muted", FontSize: 12.5}).
		SelectedStyles(giode.Styles{
			Background:  giode.Mix("theme.primary", "theme.surface", 0.16),
			BorderColor: giode.Mix("theme.primary", "theme.surface", 0.55),
			Color:       "theme.primary",
		}).
		OnClick(onClick)
	return &FilterChip{chip: c, icon: icon}
}

// View lays the chip out, on or off.
func (f *FilterChip) View(on bool) giode.Element {
	color := "theme.muted"
	if on {
		color = "theme.primary"
	}
	return f.chip.Selected(on).Icon(icons.Icon(f.icon, 14, color))
}
