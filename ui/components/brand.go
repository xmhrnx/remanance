package components

import (
	"github.com/mehran9675/giode"
	"github.com/mehran9675/remanence/assets"
	"github.com/mehran9675/remanence/ui/theme"
)

// Brand is the platter mark and the wordmark, for the title bar.
func Brand() giode.Element {
	return giode.Row(giode.Styles{Gap: theme.S2, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
		giode.Image(assets.Mark, giode.Styles{Width: 22, Height: 22}),
		giode.Text("Remanence", giode.Styles{FontFamily: theme.Display, FontSize: 14, FontWeight: giode.FontWeightSemiBold, Color: "theme.text"}),
	)
}
