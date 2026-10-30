package theme

import (
	"embed"
	"log"

	"gioui.org/font"
	"gioui.org/font/opentype"

	"github.com/mehran9675/giode"
)

//go:embed fonts/*.ttf
var fontFS embed.FS

// The type families. Inter Display is Inter cut for large sizes; JetBrains
// Mono sets offsets, sizes and hex.
const (
	Sans    = "Inter"
	Display = "Inter Display"
	Mono    = "JetBrains Mono"
)

// LoadFonts registers the embedded faces. Inter and JetBrains Mono are
// under the SIL Open Font License (fonts/*-OFL.txt).
func LoadFonts() {
	faces := []struct {
		file     string
		typeface string
		weight   font.Weight
	}{
		{"Inter-Regular.ttf", Sans, font.Normal},
		{"Inter-Medium.ttf", Sans, font.Medium},
		{"Inter-SemiBold.ttf", Sans, font.SemiBold},
		{"Inter-Bold.ttf", Sans, font.Bold},
		{"InterDisplay-SemiBold.ttf", Display, font.SemiBold},
		{"InterDisplay-Bold.ttf", Display, font.Bold},
		{"JetBrainsMono-Regular.ttf", Mono, font.Normal},
	}
	var out []font.FontFace
	for _, f := range faces {
		data, err := fontFS.ReadFile("fonts/" + f.file)
		if err != nil {
			log.Fatal(err)
		}
		face, err := opentype.Parse(data)
		if err != nil {
			log.Fatalf("%s: %v", f.file, err)
		}
		out = append(out, font.FontFace{
			Font: font.Font{Typeface: font.Typeface(f.typeface), Weight: f.weight},
			Face: face,
		})
	}
	giode.RegisterFonts(out)
}
