package screens

import "github.com/mehran9675/giode"

// Overlay lays dialogs and sheets over the whole page without taking
// any of its space.
func Overlay(layers ...giode.Element) giode.Element {
	return giode.Box(giode.Styles{
		Position: giode.PositionAbsolute,
		Left:     giode.Flush, Top: giode.Flush, Right: giode.Flush, Bottom: giode.Flush,
		ZIndex: 10,
	}, layers...)
}
