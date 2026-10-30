package components

import (
	"github.com/mehran9675/giode"
	"github.com/xmhrnx/remanance/ui/icons"
)

// CheckZone is how wide, in dp from a row's left edge, a click toggles
// the row's check mark rather than selecting the row.
const CheckZone = 40

// CheckMark draws a check box; it is drawn, not stateful, so rows built
// every frame can show one.
func CheckMark(on, hovered bool) giode.Element {
	if on {
		return giode.Box(giode.Styles{Width: 17, Height: 17, BorderRadius: 5, Background: "theme.primary",
			Align: giode.AlignCenter, Justify: giode.JustifyCenter}, icons.Icon("check", 12, "theme.onprimary"))
	}
	border := giode.Mix("theme.text", "theme.surface", 0.4)
	if hovered {
		border = giode.Mix("theme.text", "theme.surface", 0.65)
	}
	return giode.Box(giode.Styles{Width: 17, Height: 17, BorderRadius: 5, BorderWidth: 1, BorderColor: border})
}
