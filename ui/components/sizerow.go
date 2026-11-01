package components

import (
	"fmt"

	"github.com/mehran9675/giode"
	"github.com/xmhrnx/remanance/engine/recovery"
	"github.com/xmhrnx/remanance/engine/space"
	"github.com/xmhrnx/remanance/ui/icons"
	"github.com/xmhrnx/remanance/ui/theme"
)

// SizeRowHeight is the height of a row in the disk space list.
const SizeRowHeight = 48

// SizeRow is a file or folder in the disk space list: its color, name,
// size and a bar of its share of the folder it's in.
func SizeRow(n *space.Node, i int, of int64, selected bool, s giode.PressState) giode.Element {
	share := 0.0
	if of > 0 {
		share = float64(n.Size) / float64(of)
	}
	icon := "file"
	if n.IsDir {
		icon = "folder"
	}
	bg := "transparent"
	switch {
	case selected:
		bg = theme.Tint()
	case s.Hovered:
		bg = theme.Raised()
	}
	color := NodeColor(n, i)
	return giode.Row(giode.Styles{Width: -1, Height: SizeRowHeight, Gap: theme.S3, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap,
		Padding: giode.SymmetricInset(theme.S3, 0), Background: bg, BorderRadius: theme.RadiusSmall},
		giode.Box(giode.Styles{Width: 28, Height: 28, BorderRadius: 7, Align: giode.AlignCenter, Justify: giode.JustifyCenter,
			Background: giode.Mix(color, "theme.surface", 0.2)}, icons.Icon(icon, 15, color)),
		giode.Stack(giode.Styles{FlexGrow: 1, Gap: 5},
			giode.Row(giode.Styles{Width: -1, Gap: theme.S2, Wrap: giode.WrapNoWrap},
				giode.Box(giode.Styles{FlexGrow: 1}, giode.Text(n.Name, theme.Body().With(giode.Styles{FontSize: 13, MaxLines: 1, TextOverflow: giode.TextOverflowEllipsis}))),
				giode.Text(recovery.FormatBytes(n.Size), theme.Body().With(giode.Styles{FontSize: 12.5, FontWeight: giode.FontWeightMedium})),
			),
			giode.Row(giode.Styles{Width: -1, Gap: theme.S2, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
				giode.Box(giode.Styles{FlexGrow: 1}, giode.Progress(float32(share), giode.Styles{Height: 4, BorderRadius: 2, Color: color, Background: theme.Sunken()})),
				giode.Box(giode.Styles{Width: 40}, giode.Text(fmt.Sprintf("%.1f%%", share*100), theme.Caption().With(giode.Styles{FontSize: 11, TextAlign: giode.TextAlignEnd}))),
			),
		),
	)
}
