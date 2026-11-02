// Package assets embeds the app icon: the full icon for windows and
// installers, and the page mark alone for the app's own header.
package assets

import (
	"bytes"
	_ "embed"
	"image"
	"image/png"
)

//go:embed icon.png
var Icon []byte

//go:embed icon/mark.png
var markPNG []byte

// Mark is the page mark decoded, on transparency.
var Mark = func() image.Image {
	img, err := png.Decode(bytes.NewReader(markPNG))
	if err != nil {
		panic(err)
	}
	return img
}()
