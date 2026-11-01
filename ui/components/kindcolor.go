package components

import (
	"github.com/mehran9675/giode"
	"github.com/xmhrnx/remanance/engine/space"
)

// kindColors color the treemap by what files are, the same in both
// modes so a kind is recognized at a glance.
var kindColors = map[space.Kind]string{
	space.KindPicture:  "#5B9DF5",
	space.KindVideo:    "#B07CF0",
	space.KindAudio:    "#E86FA8",
	space.KindDocument: "#4FC3A1",
	space.KindArchive:  "#E8894A",
	space.KindCode:     "#8FCB5A",
	space.KindApp:      "#F2C14E",
	space.KindSystem:   "#7C8597",
	space.KindOther:    "#9AA3B5",
}

// KindColor is the color files of a kind are drawn in.
func KindColor(k space.Kind) string { return kindColors[k] }

// folderColors tint the top folders of the treemap, in turn.
var folderColors = []string{"#5B9DF5", "#4FC3A1", "#E8894A", "#B07CF0", "#F2C14E", "#E86FA8", "#8FCB5A", "#6CC4E0"}

// NodeColor is the color a file or folder is drawn in: a file by its
// kind, a folder by its place among its siblings.
func NodeColor(n *space.Node, i int) string {
	if !n.IsDir {
		return KindColor(space.KindOf(n.Name))
	}
	return folderColors[i%len(folderColors)]
}

// KindSwatch is a kind's color and name, for the legend.
func KindSwatch(k space.Kind, label string) giode.Element {
	return giode.Row(giode.Styles{Gap: 6, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
		giode.Box(giode.Styles{Width: 9, Height: 9, BorderRadius: 2, Background: KindColor(k)}),
		giode.Text(label, giode.Styles{FontSize: 12, Color: "theme.muted"}),
	)
}
