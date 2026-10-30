package theme

import (
	"gioui.org/unit"

	"github.com/mehran9675/giode"
)

// The type scale. Each returns a fresh Styles so callers can extend it
// with With.

// Hero is the largest heading, on empty and finished screens.
func Hero() giode.Styles {
	return giode.Styles{FontFamily: Display, FontSize: 30, FontWeight: giode.FontWeightBold, Color: "theme.text", LineHeight: 36}
}

// Title heads a screen.
func Title() giode.Styles {
	return giode.Styles{FontFamily: Display, FontSize: 21, FontWeight: giode.FontWeightSemiBold, Color: "theme.text"}
}

// Heading heads a card or a section.
func Heading() giode.Styles {
	return giode.Styles{FontFamily: Sans, FontSize: 15, FontWeight: giode.FontWeightSemiBold, Color: "theme.text"}
}

// Body is running text.
func Body() giode.Styles {
	return giode.Styles{FontFamily: Sans, FontSize: 13.5, Color: "theme.text", LineHeight: 20}
}

// Secondary is body text that steps back.
func Secondary() giode.Styles {
	return giode.Styles{FontFamily: Sans, FontSize: 13, Color: "theme.muted", LineHeight: 19}
}

// Caption is small print: column headers, units.
func Caption() giode.Styles {
	return giode.Styles{FontFamily: Sans, FontSize: 11.5, FontWeight: giode.FontWeightMedium, Color: "theme.muted"}
}

// Overline is a small spaced label in capitals above a group.
func Overline() giode.Styles {
	return giode.Styles{FontFamily: Sans, FontSize: 10.5, FontWeight: giode.FontWeightSemiBold, Color: "theme.muted",
		TextTransform: giode.TextTransformUppercase}
}

// Figure sets a large number: a statistic, the percentage in the ring.
func Figure(size unit.Sp) giode.Styles {
	return giode.Styles{FontFamily: Display, FontSize: size, FontWeight: giode.FontWeightSemiBold, Color: "theme.text"}
}

// Code sets offsets, sizes in bytes and hex.
func Code() giode.Styles {
	return giode.Styles{FontFamily: Mono, FontSize: 12, Color: "theme.muted"}
}
