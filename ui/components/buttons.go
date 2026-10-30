package components

import (
	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/iconbutton"
	"github.com/mehran9675/remanence/ui/icons"
	"github.com/mehran9675/remanence/ui/theme"
)

func withIcon(b *giode.ButtonElement, icon string) *giode.ButtonElement {
	if icon != "" {
		b.Icon(icons.Icon(icon, 16, ""))
	}
	return b
}

// PrimaryButton is the one main action on a screen, in the amber
// gradient.
func PrimaryButton(label, icon string) *giode.ButtonElement {
	return withIcon(giode.Button(label, giode.Styles{
		BackgroundGradient: giode.Linear(100, "#F6C878", "#E8894A"),
		Color:              "#1A1206", // dark in both modes: the gradient is light
		FontFamily:         theme.Sans, FontSize: 14, FontWeight: giode.FontWeightSemiBold,
		Padding:      giode.SymmetricInset(theme.S5, 11),
		BorderRadius: theme.Radius,
		BoxShadow:    giode.BoxShadow{Y: 6, Blur: 18, Color: "#E8894A40"},
	}), icon)
}

// SecondaryButton is an outlined button for the other actions.
func SecondaryButton(label, icon string) *giode.ButtonElement {
	return withIcon(giode.Button(label, giode.Styles{
		BorderColor: "theme.border", Color: "theme.text", Background: "theme.text",
		FontFamily: theme.Sans, FontSize: 13.5, FontWeight: giode.FontWeightMedium,
		Padding:      giode.SymmetricInset(theme.S4, 10),
		BorderRadius: theme.Radius,
	}).Variant(giode.VariantOutlined), icon)
}

// GhostButton is a quiet text button.
func GhostButton(label, icon string) *giode.ButtonElement {
	return withIcon(giode.Button(label, giode.Styles{
		Background: "theme.text", Color: "theme.muted",
		FontFamily: theme.Sans, FontSize: 13.5, FontWeight: giode.FontWeightMedium,
		Padding:      giode.SymmetricInset(theme.S3, 8),
		BorderRadius: theme.Radius,
	}).Variant(giode.VariantGhost), icon)
}

// IconAction is a square ghost button holding one icon.
func IconAction(icon, label string) *iconbutton.Button {
	return giode.IconButton(icons.Icon(icon, 18, ""), giode.Styles{Color: "theme.muted", Background: "theme.text", BorderRadius: theme.Radius}).
		Variant(giode.IconButtonGhost).Label(label)
}
