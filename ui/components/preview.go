package components

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"strings"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/scroll"
	"github.com/xmhrnx/remanance/engine/fsys"
	"github.com/xmhrnx/remanance/engine/recovery"
	"github.com/xmhrnx/remanance/ui/icons"
	"github.com/xmhrnx/remanance/ui/model"
	"github.com/xmhrnx/remanance/ui/theme"
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

func (p *Preview) body(e *fsys.Entry) giode.Element {
	if e == nil {
		return giode.Stack(giode.Styles{Width: -1, Padding: giode.UniformInset(theme.S6), Gap: theme.S3, Align: giode.AlignCenter},
			giode.Box(giode.Styles{Height: 60}),
			icons.Icon("eye", 28, giode.Mix("theme.text", "theme.surface", 0.3)),
			giode.Text("Select a file to preview it", theme.Secondary().With(giode.Styles{TextAlign: giode.TextAlignCenter})),
		)
	}
	parts := []giode.Element{p.thumbnail(e),
		giode.Stack(giode.Styles{Gap: theme.S2},
			giode.Text(e.Name, theme.Heading().With(giode.Styles{MaxLines: 3})),
			giode.Row(giode.Styles{Gap: 6}, StatePill(e.State)),
		),
		p.details(e),
	}
	if e.Note != "" {
		parts = append(parts, Banner("info", "theme.info", e.Note, nil))
	}
	if !e.IsDir && len(p.head) > 0 {
		label := "First bytes"
		var body giode.Element
		if LooksLikeText(p.head) {
			label = "Beginning of the text"
			body = giode.Text(strings.ToValidUTF8(string(p.head), "�"), theme.Code().With(giode.Styles{FontSize: 11.5, LineHeight: 17, Color: "theme.text", MaxLines: 40})).Selectable("previewtext")
		} else {
			body = HexView(p.head[:min(len(p.head), 512)])
		}
		parts = append(parts, giode.Stack(giode.Styles{Gap: theme.S2},
			giode.Text(label, theme.Overline()),
			giode.Box(giode.Styles{Width: -1, Padding: giode.UniformInset(theme.S3), Background: theme.Sunken(), BorderRadius: theme.Radius}, body),
		))
	}
	if p.err != nil {
		parts = append(parts, Banner("alert", "theme.danger", "Couldn't read this file: "+p.err.Error(), nil))
	}
	return giode.Stack(giode.Styles{Width: -1, Padding: giode.UniformInset(theme.S4), Gap: theme.S4}, parts...)
}

func (p *Preview) thumbnail(e *fsys.Entry) giode.Element {
	box := giode.Styles{Width: -1, Height: 200, BorderRadius: theme.Radius, Background: theme.Sunken(),
		Align: giode.AlignCenter, Justify: giode.JustifyCenter, Overflow: giode.OverflowHidden}
	switch {
	case p.img != nil:
		return giode.Box(box, giode.Image(p.img, giode.Styles{Width: -1, Height: -1, Fit: giode.FitContain}))
	case p.loading && model.CategoryOf(e) == model.CatPictures:
		return giode.Box(box, giode.Spinner(giode.Styles{Width: 22, Height: 22, Color: "theme.primary"}))
	}
	return giode.Box(box, FileIcon(e, 44))
}

func (p *Preview) details(e *fsys.Entry) giode.Element {
	type kv struct{ k, v string }
	var rows []kv
	if !e.IsDir {
		rows = append(rows, kv{"Size", fmt.Sprintf("%s  (%d bytes)", recovery.FormatBytes(e.Size), e.Size)})
	}
	if e.State != fsys.Existing && !e.IsDir {
		rows = append(rows, kv{"Recovery chance", fmt.Sprintf("%s, %d%%", HealthLabel(e.Health), e.Health)})
	}
	rows = append(rows, kv{"Modified", FormatTime(e.Modified)}, kv{"Created", FormatTime(e.Created)})
	if len(e.Runs) > 0 && e.Runs[0].Offset >= 0 {
		frag := ""
		if len(e.Runs) > 1 {
			frag = fmt.Sprintf(", %d fragments", len(e.Runs))
		}
		rows = append(rows, kv{"Location", fmt.Sprintf("sector %d%s", e.Runs[0].Offset/512, frag)})
	}
	rows = append(rows, kv{"Path", e.Path()})
	var out []giode.Element
	for _, r := range rows {
		out = append(out, giode.Row(giode.Styles{Width: -1, Gap: theme.S3, Wrap: giode.WrapNoWrap},
			giode.Box(giode.Styles{Width: 108}, giode.Text(r.k, theme.Secondary())),
			giode.Box(giode.Styles{FlexGrow: 1}, giode.Text(r.v, theme.Body().With(giode.Styles{FontSize: 13}))),
		))
	}
	return giode.Stack(giode.Styles{Width: -1, Gap: theme.S2}, out...)
}
