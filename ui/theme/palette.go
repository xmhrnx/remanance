// Package theme holds DataDoc's look: its palettes, type and the
// spacing every screen shares.
package theme

import "github.com/mehran9675/giode"

// Dark is the default palette: near-black ink with a warm amber accent,
// the color of the pulse in the icon.
var Dark = giode.Palette{
	Primary:    "#F4B860",
	Secondary:  "#E8894A",
	Background: "#0A0B0F",
	Surface:    "#111318",
	Text:       "#ECEEF3",
	Border:     "#232733",
	OnPrimary:  "#1A1206",
	Muted:      "#8B93A7",
	Accent:     "#E8894A",
	Success:    "#4ADE9B",
	Warning:    "#F5C451",
	Danger:     "#F2706B",
	Info:       "#7CB7FF",
}

// Light is a warm paper palette with the same accent, deepened to keep
// its contrast on white.
var Light = giode.Palette{
	Primary:    "#C9771F",
	Secondary:  "#B55A1E",
	Background: "#F7F5F1",
	Surface:    "#FFFFFF",
	Text:       "#16171C",
	Border:     "#E4E0D8",
	OnPrimary:  "#FFFFFF",
	Muted:      "#6B7080",
	Accent:     "#B55A1E",
	Success:    "#178A5A",
	Warning:    "#B07A0C",
	Danger:     "#C8423C",
	Info:       "#2F6FCC",
}

// Apply installs both palettes and starts in dark mode.
func Apply() {
	giode.SetPalette(giode.ModeDark, Dark)
	giode.SetPalette(giode.ModeLight, Light)
	giode.SetMode(giode.ModeDark)
}

// Raised is a surface one step above the panels: hovered rows, inputs,
// the inside of cards. It's mixed opaque, since translucent fills blend
// in linear light and come out too bright.
func Raised() string { return giode.Mix("theme.text", "theme.surface", 0.045) }

// Sunken sits below the surface: the sector map's well, code blocks.
func Sunken() string {
	if giode.CurrentMode() == giode.ModeLight {
		return giode.Mix("theme.text", "theme.background", 0.045)
	}
	return giode.Mix("#000000", "theme.background", 0.25)
}

// Tint is the accent faintly washed over the surface, for a selected row.
func Tint() string { return giode.Mix("theme.primary", "theme.surface", 0.14) }

// Hairline is a divider fainter than the border.
func Hairline() string { return giode.Mix("theme.border", "theme.surface", 0.6) }
