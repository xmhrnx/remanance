package screens

import (
	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/dropdown"
	"github.com/mehran9675/giode/components/iconbutton"
	"github.com/mehran9675/giode/components/input"
	"github.com/mehran9675/giode/components/segmented"
	"github.com/mehran9675/giode/components/sheet"
	"github.com/xmhrnx/remanance/engine/scan"
	"github.com/xmhrnx/remanance/locales"
	"github.com/xmhrnx/remanance/ui/components"
	"github.com/xmhrnx/remanance/ui/model"
	"github.com/xmhrnx/remanance/ui/theme"
)

// Results browses what a scan found and recovers it: the folder tree on
// the left, the open folder's files in the middle, the selected file on
// the right. On a phone the tree and preview step aside: folders open in
// place and a file's preview slides up in a sheet.
type Results struct {
	m *model.Model

	tree      *giode.TreeViewElement
	treeRoots []giode.TreeNode
	treeFrom  []*model.Node
	shownDir  *model.Node

	list      *giode.VirtualListElement
	checkAll  *giode.PressableElement
	search    *input.Input
	show      *segmented.Control
	sort      *dropdown.Select
	cats      []*components.FilterChip
	catValues []model.Category

	preview     *components.Preview
	previewOpen bool
	sheet       *sheet.Sheet

	up      *iconbutton.Button
	recover *giode.ButtonElement
	newScan *giode.ButtonElement
	clear   *giode.ButtonElement
	deep    *giode.ButtonElement
	dialog  *RecoverDialog
}

// NewResults builds the results screen.
func NewResults(m *model.Model) *Results {
	r := &Results{m: m, preview: components.NewPreview(m.App)}
	r.tree = giode.TreeView(func() []giode.TreeNode { return r.treeRoots }, r.treeRow,
		giode.Styles{Height: 32, Color: "theme.muted", BorderColor: "theme.primary/0.10", ScrollBar: giode.ScrollBar{AutoHide: true}}).
		OnSelect(func(n giode.TreeNode) {
			r.shownDir = n.(*model.Node)
			m.OpenFolder(r.shownDir)
		})
	r.list = giode.VirtualList(func() int { return len(m.Rows()) }, components.RowHeight, r.row,
		giode.Styles{ScrollBar: giode.ScrollBar{AutoHide: true}, BorderColor: "theme.primary"}).
		ClickFocusesRow(false)
	r.checkAll = giode.Pressable(func(s giode.PressState) giode.Element {
		return components.CheckMark(r.allChecked(), s.Hovered)
	}).OnClick(func(giode.ClickEvent) { m.CheckAll() }).Label(locales.T("results.check_every_file_listed"))
	r.search = giode.Input.Text(locales.T("results.search_names"), giode.Styles{FontSize: 13, Padding: giode.SymmetricInset(10, 8), BorderRadius: theme.Radius}).
		OnChange(func(t string) { f := m.Filter; f.Query = t; m.SetFilter(f) })
	r.show = giode.Input.Segmented(locales.T("results.deleted"), locales.T("results.all_files")).Styles(segmentStyles().With(giode.Styles{Width: 180, FontSize: 12.5})).
		OnChange(func(i int) { f := m.Filter; f.Show = model.Show(i); m.SetFilter(f) })
	var sorts []string
	for _, c := range model.SortChoices {
		sorts = append(sorts, c.Label)
	}
	r.sort = giode.Input.Select(giode.Styles{FontSize: 13, BorderRadius: theme.Radius}, sorts...).Sheet(giode.SheetCompact).
		OnChange(func(i int) { m.Order = model.SortChoices[i].Order })
	r.catValues = []model.Category{model.CatPictures, model.CatVideo, model.CatAudio, model.CatDocuments, model.CatArchives}
	catIcons := []string{"picture", "video", "music", "document", "archive"}
	for i, c := range r.catValues {
		c, icon := c, catIcons[i]
		r.cats = append(r.cats, components.NewFilterChip(c.Label(), icon, func() {
			f := m.Filter
			if f.Category == c {
				f.Category = model.CatAny
			} else {
				f.Category = c
			}
			m.SetFilter(f)
		}))
	}
	r.sheet = giode.Sheet(&r.previewOpen, func() giode.Element {
		return giode.Box(giode.Styles{Width: -1, Height: 520}, r.preview.View(m.Dev, m.Preview))
	}).Detents(giode.DetentMedium, giode.DetentLarge)
	r.up = components.IconAction("arrow-left", locales.T("results.up_one_folder")).OnClick(func() {
		if m.Folder != nil && m.Folder.Parent != nil {
			m.OpenFolder(m.Folder.Parent)
		}
	})
	r.dialog = NewRecoverDialog(m)
	r.recover = components.PrimaryButton(locales.T("results.recover"), "recover").OnClick(r.dialog.Open)
	r.newScan = components.GhostButton(locales.T("results.new_scan"), "arrow-left").OnClick(m.NewScan)
	r.clear = components.GhostButton(locales.T("results.clear"), "").OnClick(m.ClearChecked)
	r.deep = components.PrimaryButton(locales.T("results.run_a_deep_scan"), "radar").OnClick(func() { m.StartScan(scan.Deep) })
	return r
}

func (r *Results) allChecked() bool {
	rows := r.m.Rows()
	if len(rows) == 0 {
		return false
	}
	for _, e := range rows {
		if !r.m.IsChecked(e) {
			return false
		}
	}
	return true
}

// View lays out the screen.
func (r *Results) View() giode.Element {
	r.syncTree()
	files, size := r.m.Selection()
	r.recover.Disabled(files == 0)
	if files > 0 {
		r.recover.Label(locales.T("results.recover_n", "n", files))
	} else {
		r.recover.Label(locales.T("results.recover"))
	}
	compact := giode.Compact()
	var body giode.Element
	if compact {
		body = r.table(true)
	} else {
		body = giode.Row(giode.Styles{Width: -1, FlexGrow: 1, Wrap: giode.WrapNoWrap},
			r.sidebar(),
			giode.Box(giode.Styles{FlexGrow: 1, Height: -1}, r.table(false)),
			giode.Box(giode.Styles{Width: 320, Height: -1, Background: "theme.surface", BorderWidth: 1, BorderColor: "theme.border"},
				r.preview.View(r.m.Dev, r.m.Preview)),
		)
	}
	// The sheet and dialog lay themselves over the window.
	return giode.Stack(giode.Styles{Width: -1, Height: -1},
		r.summary(compact),
		body,
		r.bar(files, size, compact),
		r.sheet,
		r.dialog.View(),
	)
}
