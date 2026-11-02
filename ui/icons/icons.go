// Package icons is DataDoc's own stroke icon set, drawn on a 24dp grid
// with 1.75 strokes and round joins.
package icons

import (
	"embed"
	"log"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/styles/properties"
)

//go:embed svg/*.svg
var svgFS embed.FS

// Register adds every icon to giode's registry under its file name.
func Register() {
	if err := giode.RegisterIconsFS(svgFS, "svg"); err != nil {
		log.Fatal(err)
	}
}

// Icon draws a named icon at size dp in color; an empty color follows the
// surrounding text.
func Icon(name string, size int, color string) giode.Element {
	return giode.Icon(name, giode.Styles{Width: properties.Width(size), Height: properties.Height(size), Color: color})
}
