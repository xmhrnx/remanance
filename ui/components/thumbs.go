package components

import (
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"sync"

	"golang.org/x/image/draw"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/styles/properties"
	"github.com/xmhrnx/remanance/ui/theme"
)

// thumbSize is the longest side of a thumbnail, in pixels.
const thumbSize = 192

// Thumbs loads small pictures of photos on disk, a few at a time in the
// background, and keeps them.
type Thumbs struct {
	app  *giode.App
	mu   sync.Mutex
	imgs map[string]image.Image
	busy map[string]bool
	sem  chan struct{}
}

// NewThumbs returns an empty cache.
func NewThumbs(app *giode.App) *Thumbs {
	return &Thumbs{app: app, imgs: map[string]image.Image{}, busy: map[string]bool{}, sem: make(chan struct{}, 3)}
}

// Clear forgets every thumbnail.
func (t *Thumbs) Clear() {
	t.mu.Lock()
	t.imgs = map[string]image.Image{}
	t.mu.Unlock()
}

// View shows the photo at path size dp square, loading it if needed.
func (t *Thumbs) View(path string, size int) giode.Element {
	t.mu.Lock()
	img, ok := t.imgs[path]
	if !ok && !t.busy[path] {
		t.busy[path] = true
		go t.load(path)
	}
	t.mu.Unlock()
	box := giode.Styles{Width: properties.Width(size), Height: properties.Height(size), BorderRadius: theme.RadiusSmall, Background: theme.Sunken(), Overflow: giode.OverflowHidden,
		Align: giode.AlignCenter, Justify: giode.JustifyCenter}
	if img == nil {
		if ok {
			return giode.Box(box, FileIconNamed(path, size/2))
		}
		return giode.Box(box, giode.Spinner(giode.Styles{Width: 14, Height: 14, Color: "theme.muted"}))
	}
	return giode.Box(box, giode.Image(img, giode.Styles{Width: -1, Height: -1, Fit: giode.FitCover}))
}

func (t *Thumbs) load(path string) {
	t.sem <- struct{}{}
	img := decodeThumb(path)
	<-t.sem
	t.app.Post(func() {
		t.mu.Lock()
		t.imgs[path] = img
		delete(t.busy, path)
		t.mu.Unlock()
	})
}

// decodeThumb decodes a picture and shrinks it, or returns nil.
func decodeThumb(path string) image.Image {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil || cfg.Width*cfg.Height > 80_000_000 {
		return nil
	}
	f.Seek(0, 0)
	src, _, err := image.Decode(f)
	if err != nil {
		return nil
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= thumbSize && h <= thumbSize {
		return src
	}
	if w > h {
		w, h = thumbSize, h*thumbSize/w
	} else {
		w, h = w*thumbSize/h, thumbSize
	}
	dst := image.NewRGBA(image.Rect(0, 0, max(w, 1), max(h, 1)))
	draw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, b, draw.Src, nil)
	return dst
}
