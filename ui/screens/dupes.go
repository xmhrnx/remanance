package screens

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/iconbutton"
	"github.com/mehran9675/giode/components/scroll"
	"github.com/mehran9675/giode/components/segmented"
	"github.com/mehran9675/giode/components/toggle"
	"github.com/xmhrnx/remanance/engine/dupes"
	"github.com/xmhrnx/remanance/engine/recovery"
	"github.com/xmhrnx/remanance/engine/trash"
	"github.com/xmhrnx/remanance/locales"
	"github.com/xmhrnx/remanance/ui/components"
	"github.com/xmhrnx/remanance/ui/icons"
	"github.com/xmhrnx/remanance/ui/model"
	"github.com/xmhrnx/remanance/ui/theme"
)

// dupeRowHeight is the height of a row in the results: a group's header
// or one of its files.
const dupeRowHeight = 64

// thresholds are how alike photos must be, by the strictness chosen.
var thresholds = []int{3, 6, 10}

// strictNotes say what each strictness finds.
var strictNotes = []string{
	locales.T("dupes.only_the_same_picture_resized_or"),
	locales.T("dupes.also_lightly_edited_copies_and_burst"),
	locales.T("dupes.also_photos_of_the_same_scene"),
}

// Dupes finds files that are the same and photos and videos that look
// the same, and moves the extra copies to the trash.
type Dupes struct {
	m       *model.Model
	chooser *FolderChooser
	intro   *scroll.Scroll
	similar *toggle.Switch
	strict  *segmented.Control
	start   *giode.ButtonElement
	stop    *giode.ButtonElement
	back    *iconbutton.Button
	show    *segmented.Control
	list    *giode.VirtualListElement
	remove  *giode.ButtonElement
	confirm *Confirm
	thumbs  *components.Thumbs

	// rows flattens the groups shown: a header, then its files.
	rows    []dupeRow
	rowsFor []dupes.Group
	rowsOf  int
}

type dupeRow struct {
	group int // into the model's groups
	file  int // -1 for the group's header
}

// NewDupes builds the duplicates screen.
func NewDupes(m *model.Model) *Dupes {
	d := &Dupes{m: m, confirm: NewConfirm(), thumbs: components.NewThumbs(m.App)}
	d.chooser = NewFolderChooser(m, "dupes-place", locales.T("dupes.add"), m.AddDupesRoot)
	d.intro = giode.Scroll(d.introView, giode.Styles{ScrollBar: giode.ScrollBar{AutoHide: true}})
	d.similar = giode.Input.Switch(locales.T("dupes.also_find_similar_photos_and_videos"), giode.Styles{FontSize: 13.5, Color: "theme.text", Background: "theme.primary"}).
		OnChange(func(on bool) { m.Dupes.Similar = on })
	d.similar.SetChecked(m.Dupes.Similar)
	d.strict = giode.Input.Segmented(locales.T("dupes.strict"), locales.T("dupes.balanced"), locales.T("dupes.loose")).Styles(segmentStyles().With(giode.Styles{Width: -1, MaxWidth: 360, FontSize: 12.5})).
		OnChange(func(i int) { m.Dupes.Threshold = thresholds[i] })
	d.strict.SetSelected(1)
	d.start = components.PrimaryButton(locales.T("dupes.find_duplicates"), "search").OnClick(func() {
		d.thumbs.Clear()
		m.FindDupes()
	})
	d.stop = components.SecondaryButton(locales.T("dupes.stop"), "stop").OnClick(m.CancelDupes)
	d.back = components.IconAction("arrow-left", locales.T("dupes.new_search")).OnClick(func() {
		m.Dupes.Groups, m.Dupes.Done, m.Dupes.Err = nil, "", nil
	})
	d.show = giode.Input.Segmented(locales.T("dupes.all"), locales.T("dupes.exact_copies"), locales.T("dupes.similar")).Styles(segmentStyles().With(giode.Styles{Width: 320, FontSize: 12.5}))
	d.list = giode.VirtualList(func() int { return len(d.visible()) }, dupeRowHeight, d.row,
		giode.Styles{ScrollBar: giode.ScrollBar{AutoHide: true}})
	d.remove = components.PrimaryButton(locales.T("dupes.move_to_trash"), "trash").OnClick(d.askRemove)
	return d
}

func (d *Dupes) askRemove() {
	n, size := d.m.DupesChecked()
	if n == 0 {
		return
	}
	title := locales.T("dupes.trash_title", "files", count(n, "file"))
	msg := locales.T("dupes.trash_message", "size", recovery.FormatBytes(size))
	yes := locales.T("dupes.move_to_trash")
	if !trash.Supported {
		title = locales.T("dupes.delete_title", "files", count(n, "file"))
		msg = locales.T("dupes.delete_message", "size", recovery.FormatBytes(size))
		yes = locales.T("dupes.delete")
	}
	d.confirm.Ask(title, msg, yes, d.m.RemoveDupes)
}

// visible flattens the groups the filter shows into rows, again only
// when they change.
func (d *Dupes) visible() []dupeRow {
	gs := d.m.Dupes.Groups
	show := d.show.Selected()
	if len(gs) == len(d.rowsFor) && (len(gs) == 0 || &gs[0] == &d.rowsFor[0]) && show == d.rowsOf && d.rows != nil && sameSizes(gs, d.rowsFor) {
		return d.rows
	}
	d.rows = d.rows[:0]
	for gi, g := range gs {
		if show == 1 && g.Kind != dupes.Exact || show == 2 && g.Kind == dupes.Exact {
			continue
		}
		d.rows = append(d.rows, dupeRow{gi, -1})
		for fi := range g.Files {
			d.rows = append(d.rows, dupeRow{gi, fi})
		}
	}
	d.rowsFor, d.rowsOf = gs, show
	return d.rows
}

func sameSizes(a, b []dupes.Group) bool {
	for i := range a {
		if len(a[i].Files) != len(b[i].Files) {
			return false
		}
	}
	return true
}

func (d *Dupes) row(i int) giode.Element {
	rows := d.visible()
	if i >= len(rows) {
		return giode.Box(giode.Styles{})
	}
	r := rows[i]
	g := d.m.Dupes.Groups[r.group]
	if r.file < 0 {
		return d.header(g)
	}
	f := g.Files[r.file]
	p := giode.Keyed("dupe-row", f.Path, func() *giode.PressableElement {
		return giode.Pressable(func(giode.PressState) giode.Element { return nil })
	})
	return p.Content(func(s giode.PressState) giode.Element {
		return d.fileRow(g, f, s)
	}).OnClick(func(giode.ClickEvent) { d.m.ToggleDupe(g, f.Path) }).Label(filepath.Base(f.Path))
}

func (d *Dupes) header(g dupes.Group) giode.Element {
	icon, color := "copy", "theme.info"
	detail := locales.T("dupes.copies_each", "copies", count(len(g.Files), "copy"), "size", recovery.FormatBytes(g.Files[0].Size))
	switch g.Kind {
	case dupes.SimilarPhoto:
		icon, color = "picture", "#E58FB0"
		detail = count(len(g.Files), "photo")
		if g.Distance == 0 {
			detail = locales.T("dupes.same_picture", "photos", detail)
		}
	case dupes.SimilarVideo:
		icon, color = "video", "#A98BF0"
		detail = locales.T("dupes.videos_long", "videos", count(len(g.Files), "video"), "length", g.Files[0].Duration.Round(time.Second))
	}
	return giode.Row(giode.Styles{Width: -1, Height: -1, Gap: theme.S3, Align: giode.AlignEnd, Wrap: giode.WrapNoWrap,
		Padding: giode.Inset{Left: theme.S2, Right: theme.S3, Bottom: theme.S2}},
		icons.Icon(icon, 16, color),
		giode.Text(label(g.Kind.Label()), theme.Body().With(giode.Styles{FontSize: 13, FontWeight: giode.FontWeightSemiBold})),
		giode.Box(giode.Styles{FlexGrow: 1}, giode.Text(detail, theme.Caption())),
		giode.Text(locales.T("dupes.extra", "size", recovery.FormatBytes(g.Wasted())), theme.Caption().With(giode.Styles{Color: "theme.primary"})),
	)
}

func (d *Dupes) fileRow(g dupes.Group, f dupes.File, s giode.PressState) giode.Element {
	checked := d.m.Dupes.Remove[f.Path]
	bg := "theme.surface"
	switch {
	case checked:
		bg = giode.Mix("theme.danger", "theme.surface", 0.07)
	case s.Hovered:
		bg = theme.Raised()
	}
	var thumb giode.Element
	if g.Kind != dupes.SimilarVideo && isPicture(f.Path) {
		thumb = d.thumbs.View(f.Path, 48)
	} else {
		thumb = giode.Box(giode.Styles{Width: 48, Height: 48, BorderRadius: theme.RadiusSmall, Background: theme.Sunken(),
			Align: giode.AlignCenter, Justify: giode.JustifyCenter}, components.FileIconNamed(f.Path, 22))
	}
	var facts []string
	facts = append(facts, recovery.FormatBytes(f.Size))
	if f.Width > 0 {
		facts = append(facts, fmt.Sprintf("%d×%d", f.Width, f.Height))
	}
	facts = append(facts, f.Modified.Format("Jan 2, 2006"))
	tags := []giode.Element{}
	if f.Keep {
		tags = append(tags, components.Tag(locales.T("dupes.suggested_to_keep"), "theme.success"))
	}
	if checked {
		tags = append(tags, components.Tag(locales.T("dupes.will_be_removed"), "theme.danger"))
	}
	return giode.Row(giode.Styles{Width: -1, Height: dupeRowHeight - 4, Gap: theme.S3, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap,
		Padding: giode.SymmetricInset(theme.S3, 0), Background: bg, BorderRadius: theme.RadiusSmall, BorderWidth: 1, BorderColor: theme.Hairline()},
		components.CheckMark(checked, s.Hovered),
		thumb,
		giode.Stack(giode.Styles{FlexGrow: 1, Gap: 3},
			giode.Text(filepath.Base(f.Path), theme.Body().With(giode.Styles{FontSize: 13, FontWeight: giode.FontWeightMedium, MaxLines: 1, TextOverflow: giode.TextOverflowEllipsis})),
			giode.Text(filepath.Dir(f.Path), theme.Caption().With(giode.Styles{FontSize: 11.5, MaxLines: 1, TextOverflow: giode.TextOverflowEllipsis})),
		),
		giode.Row(giode.Styles{Gap: theme.S2, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap}, tags...),
		giode.Box(giode.Styles{Width: 190}, giode.Text(join(facts), theme.Caption().With(giode.Styles{TextAlign: giode.TextAlignEnd, MaxLines: 1}))),
	)
}

func join(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += locales.T("common.separator")
		}
		out += p
	}
	return out
}

func isPicture(p string) bool {
	switch filepath.Ext(p) {
	case ".jpg", ".jpeg", ".png", ".gif", ".JPG", ".JPEG", ".PNG", ".GIF", ".bmp", ".webp":
		return true
	}
	return false
}

// View lays out the screen.
func (d *Dupes) View() giode.Element {
	st := &d.m.Dupes
	var body giode.Element
	switch {
	case st.Running:
		body = d.running()
	case st.Groups != nil:
		body = d.results()
	default:
		body = d.intro
	}
	return giode.Stack(giode.Styles{Width: -1, Height: -1}, giode.Box(giode.Styles{Width: -1, FlexGrow: 1}, body), d.confirm.View())
}

func (d *Dupes) introView() giode.Element {
	st := &d.m.Dupes
	compact := giode.Compact()
	kids := []giode.Element{
		toolHeader("copy", locales.T("dupes.find_duplicates_and_look_alikes"),
			locales.T("dupes.finds_files_that_are_exact_copies")),
	}
	if st.Err != nil {
		kids = append(kids, components.Banner("alert", "theme.danger", st.Err.Error(), nil))
	}
	var chips []giode.Element
	for _, r := range st.Roots {
		r := r
		x := giode.Keyed("dupes-root", r, func() *iconbutton.Button {
			return components.IconAction("x", locales.T("dupes.don_t_look_here")).OnClick(func() { d.m.RemoveDupesRoot(r) })
		})
		chips = append(chips, giode.Row(giode.Styles{Gap: theme.S2, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, Padding: giode.Inset{Left: theme.S3, Right: 4},
			Height: 40, Background: theme.Raised(), BorderRadius: 20, BorderWidth: 1, BorderColor: "theme.border"},
			icons.Icon("folder", 16, "theme.primary"),
			giode.Text(r, theme.Body().With(giode.Styles{FontSize: 13, MaxLines: 1})),
			x))
	}
	if len(chips) == 0 {
		chips = append(chips, giode.Text(locales.T("dupes.choose_one_or_more_folders_below"), theme.Secondary()))
	}
	d.start.Disabled(len(st.Roots) == 0)
	options := []giode.Element{d.similar}
	if st.Similar {
		options = append(options, giode.Stack(giode.Styles{Width: -1, Gap: theme.S2},
			giode.Text(locales.T("dupes.how_alike_photos_must_be"), theme.Caption()), d.strict,
			giode.Text(strictNotes[d.strict.Selected()], theme.Caption())))
	}
	kids = append(kids,
		section(locales.T("dupes.look_in"), giode.Row(giode.Styles{Width: -1, Gap: theme.S2}, chips...)),
		section(locales.T("dupes.add_a_folder"), d.chooser.View(compact)),
		section(locales.T("dupes.options"), giode.Stack(giode.Styles{Gap: theme.S3}, options...)),
		giode.Row(giode.Styles{Gap: theme.S3, Align: giode.AlignCenter}, d.start,
			giode.Text(locales.T("dupes.photos_jpeg_png_gif_webp_bmp"), theme.Caption())),
	)
	return page(compact, kids...)
}

func (d *Dupes) running() giode.Element {
	p := d.m.Dupes.Progress
	frac := float32(0)
	if p.Of > 0 {
		frac = float32(p.Done) / float32(p.Of)
	}
	detail := locales.T("dupes.listed", "files", count(p.Files, "file"))
	if p.Of > 0 {
		detail = locales.T("dupes.progress", "done", p.Done, "of", p.Of, "groups", count(p.Groups, "group"))
	}
	return giode.Stack(giode.Styles{Width: -1, Height: -1, Align: giode.AlignCenter, Justify: giode.JustifyCenter, Gap: theme.S4, Padding: giode.UniformInset(theme.S6)},
		giode.Spinner(giode.Styles{Width: 36, Height: 36, Color: "theme.primary"}),
		giode.Text(locales.T("common.ongoing", "what", label(p.Phase.Label())), theme.Title()),
		giode.Box(giode.Styles{Width: 360}, giode.Progress(frac, giode.Styles{Height: 6, BorderRadius: 3, Color: "theme.primary", Background: theme.Sunken()})),
		giode.Text(detail, theme.Secondary()),
		d.stop,
	)
}

func (d *Dupes) results() giode.Element {
	st := &d.m.Dupes
	compact := giode.Compact()
	var wasted int64
	for _, g := range st.Groups {
		wasted += g.Wasted()
	}
	summary := locales.T("dupes.summary", "groups", count(len(st.Groups), "group"), "size", recovery.FormatBytes(wasted))
	top := []giode.Element{d.back, giode.Box(giode.Styles{FlexGrow: 1}, giode.Text(summary, theme.Heading().With(giode.Styles{FontSize: 15})))}
	if !compact {
		top = append(top, d.show)
	}
	bar := giode.Row(giode.Styles{Width: -1, Gap: theme.S3, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, Padding: giode.SymmetricInset(theme.S4, theme.S3),
		Background: "theme.surface", BorderWidth: 1, BorderColor: "theme.border"}, top...)
	var notes []giode.Element
	if st.Done != "" {
		notes = append(notes, components.Banner("check", "theme.success", st.Done, nil))
	}
	if st.Err != nil {
		notes = append(notes, components.Banner("alert", "theme.danger", st.Err.Error(), nil))
	}
	var body giode.Element
	if len(st.Groups) == 0 {
		body = components.EmptyState("check", locales.T("dupes.no_duplicates_found"), locales.T("dupes.every_file_in_these_folders_is"), nil)
	} else {
		body = giode.Box(giode.Styles{Width: -1, Height: -1, Padding: giode.SymmetricInset(theme.S4, 0)}, d.list)
	}
	n, size := d.m.DupesChecked()
	d.remove.Disabled(n == 0 || st.Removing)
	label := locales.T("dupes.move_to_trash")
	if !trash.Supported {
		label = locales.T("dupes.delete")
	}
	if st.Removing {
		label = locales.T("dupes.removing")
	}
	d.remove.Label(label)
	foot := giode.Row(giode.Styles{Width: -1, Gap: theme.S3, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, Padding: giode.SymmetricInset(theme.S4, theme.S3),
		Background: "theme.surface", BorderWidth: 1, BorderColor: "theme.border"},
		giode.Box(giode.Styles{FlexGrow: 1}, giode.Text(locales.T("dupes.checked", "files", count(n, "file"), "size", recovery.FormatBytes(size)), theme.Secondary())),
		d.remove)
	return giode.Stack(giode.Styles{Width: -1, Height: -1},
		append(append([]giode.Element{bar}, notes...), giode.Box(giode.Styles{Width: -1, FlexGrow: 1}, body), foot)...)
}
