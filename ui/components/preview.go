package components

import (
	"bytes"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/scroll"
	"github.com/xmhrnx/remanance/engine/fsys"
	"github.com/xmhrnx/remanance/ui/model"
)

// maxPreviewImage is the largest picture decoded for a thumbnail.
const maxPreviewImage = 40 << 20

// headBytes is how much of a file the hex and text views show.
const headBytes = 2048

// Preview is the panel showing the selected file: a picture's thumbnail,
// its details and its first bytes. Create it once; it reads the file in
// the background whenever the selection changes.
type Preview struct {
	app     *giode.App
	entry   *fsys.Entry
	img     image.Image
	head    []byte
	loading bool
	err     error
	view    *scroll.Scroll
	content func() giode.Element
}

// NewPreview returns an empty preview panel.
func NewPreview(app *giode.App) *Preview {
	p := &Preview{app: app}
	p.view = giode.Scroll(func() giode.Element { return p.content() }, giode.Styles{ScrollBar: giode.ScrollBar{AutoHide: true}})
	return p
}

// load reads e from dev, unless it's the file already shown.
func (p *Preview) load(dev io.ReaderAt, e *fsys.Entry) {
	if e == p.entry {
		return
	}
	p.entry, p.img, p.head, p.err = e, nil, nil, nil
	if e == nil || e.IsDir || dev == nil {
		return
	}
	p.loading = true
	go func() {
		var img image.Image
		var head []byte
		r, err := fsys.Open(dev, e)
		if err == nil {
			head = make([]byte, min(headBytes, e.Size))
			n, _ := r.ReadAt(head, 0)
			head = head[:n]
			if model.CategoryOf(e) == model.CatPictures && e.Size <= maxPreviewImage {
				data := make([]byte, e.Size)
				if n, _ := r.ReadAt(data, 0); n > 0 {
					img, _, _ = image.Decode(bytes.NewReader(data[:n]))
				}
			}
		}
		p.app.Post(func() {
			if p.entry == e {
				p.img, p.head, p.err, p.loading = img, head, err, false
			}
		})
	}()
}

// View shows e, read from dev.
func (p *Preview) View(dev io.ReaderAt, e *fsys.Entry) giode.Element {
	p.load(dev, e)
	p.content = func() giode.Element { return p.body(e) }
	return giode.Box(giode.Styles{Width: -1, Height: -1}, p.view)
}
