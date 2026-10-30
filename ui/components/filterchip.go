package components

import (
	"github.com/mehran9675/giode"
	"github.com/mehran9675/remanence/ui/icons"
	"github.com/mehran9675/remanence/ui/theme"
)

// FilterChip is a rounded toggle in the filter row: an outline at rest,
// a wash of the accent when on.
func FilterChip(label, icon string, on bool, s giode.PressState) giode.Element {
	bg, border, color := "", "theme.border", "theme.muted"
	switch {
	case on:
		bg, border, color = giode.Mix("theme.primary", "theme.surface", 0.16), giode.Mix("theme.primary", "theme.surface", 0.55), "theme.primary"
	case s.Hovered:
		bg, border, color = theme.Raised(), giode.Mix("theme.text", "theme.border", 0.2), "theme.text"
	}
	kids := []giode.Element{}
	if icon != "" {
		kids = append(kids, icons.Icon(icon, 14, color))
	}
	kids = append(kids, giode.Text(label, giode.Styles{FontFamily: theme.Sans, FontSize: 12.5, FontWeight: giode.FontWeightMedium, Color: color}))
	return giode.Row(giode.Styles{Gap: 6, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, Height: 30,
		Padding: giode.SymmetricInset(12, 0), BorderRadius: 15, BorderWidth: 1, BorderColor: border, Background: bg,
		Cursor: giode.CursorPointer}, kids...)
}
