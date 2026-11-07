package screens

import (
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
	"github.com/xmhrnx/remanance/ui/model"
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
