package components

import (
	"strconv"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/styles/properties"
	"github.com/xmhrnx/remanance/ui/theme"
)

// HealthColor grades a recovery chance: green when the file's space is
// untouched, amber when some of it may be reused, red when most is.
func HealthColor(h uint8) string {
	switch {
	case h >= 90:
		return "theme.success"
	case h >= 50:
		return "theme.warning"
	}
	return "theme.danger"
}

// HealthLabel names a recovery chance.
func HealthLabel(h uint8) string {
	switch {
	case h >= 90:
		return "Good"
	case h >= 50:
		return "Fair"
	}
	return "Poor"
}

// HealthBar shows a recovery chance as a short meter with its percentage.
func HealthBar(h uint8) giode.Element {
	const w = 44
	fill := max(3, int(h)*w/100)
	return giode.Row(giode.Styles{Gap: theme.S2, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
		giode.Box(giode.Styles{Width: w, Height: 5, BorderRadius: 3, Background: giode.Mix("theme.text", "theme.surface", 0.08)},
			giode.Box(giode.Styles{Width: properties.Width(fill), Height: 5, BorderRadius: 3, Background: HealthColor(h)})),
		giode.Text(strconv.Itoa(int(h))+"%", theme.Code().With(giode.Styles{FontSize: 11})),
	)
}
