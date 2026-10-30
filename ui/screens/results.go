package screens

import (
	"fmt"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/dropdown"
	"github.com/mehran9675/giode/components/iconbutton"
	"github.com/mehran9675/giode/components/input"
	"github.com/mehran9675/giode/components/segmented"
	"github.com/mehran9675/giode/components/sheet"
	"github.com/xmhrnx/remanance/engine/disk"
	"github.com/xmhrnx/remanance/engine/fsys"
	"github.com/xmhrnx/remanance/engine/recovery"
	"github.com/xmhrnx/remanance/engine/scan"
	"github.com/xmhrnx/remanance/ui/components"
	"github.com/xmhrnx/remanance/ui/icons"
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
	}).OnClick(func(giode.ClickEvent) { m.CheckAll() }).Label("Check every file listed")
	r.search = giode.Input.Text("Search names", giode.Styles{FontSize: 13, Padding: giode.SymmetricInset(10, 8), BorderRadius: theme.Radius}).
		OnChange(func(t string) { f := m.Filter; f.Query = t; m.SetFilter(f) })
	r.show = giode.Input.Segmented("Deleted", "All files").Styles(segmentStyles().With(giode.Styles{Width: 180, FontSize: 12.5})).
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
		r.cats = append(r.cats, components.NewFilterChip(string(c), icon, func() {
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
	r.up = components.IconAction("arrow-left", "Up one folder").OnClick(func() {
		if m.Folder != nil && m.Folder.Parent != nil {
			m.OpenFolder(m.Folder.Parent)
		}
	})
	r.dialog = NewRecoverDialog(m)
	r.recover = components.PrimaryButton("Recover", "recover").OnClick(r.dialog.Open)
	r.newScan = components.GhostButton("New scan", "arrow-left").OnClick(m.NewScan)
	r.clear = components.GhostButton("Clear", "").OnClick(m.ClearChecked)
	r.deep = components.PrimaryButton("Run a deep scan", "radar").OnClick(func() { m.StartScan(scan.Deep) })
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

// syncTree follows the model: new results replace the tree's roots, and
// a folder opened from the table is selected and revealed in the tree.
func (r *Results) syncTree() {
	if len(r.treeFrom) != len(r.m.Roots) || (len(r.treeFrom) > 0 && r.treeFrom[0] != r.m.Roots[0]) {
		r.treeFrom = r.m.Roots
		r.treeRoots = r.treeRoots[:0]
		for _, n := range r.m.Roots {
			r.treeRoots = append(r.treeRoots, n)
		}
		if len(r.m.Roots) > 0 {
			r.tree.Expand(r.m.Roots[0], true)
		}
		r.shownDir = nil
	}
	if f := r.m.Folder; f != nil && f != r.shownDir {
		r.shownDir = f
		for _, a := range f.Trail()[:len(f.Trail())-1] {
			r.tree.Expand(a, true)
		}
		r.tree.Select(f)
		r.tree.Reveal(f)
	}
}

func (r *Results) treeRow(tn giode.TreeNode, s giode.TreeRowState) giode.Element {
	n := tn.(*model.Node)
	icon, color := "folder", giode.Mix("theme.text", "theme.surface", 0.55)
	if n.Entry.State != fsys.Existing {
		color = "theme.primary"
	}
	title := theme.Body().With(giode.Styles{FontSize: 13, TextOverflow: giode.TextOverflowEllipsis})
	if s.Selected {
		title.Color = "theme.text"
		title.FontWeight = giode.FontWeightMedium
	}
	var extra []giode.Element
	switch n.Kind {
	case model.KindVolume:
		icon, color = "drive", "theme.text"
		title.FontWeight = giode.FontWeightSemiBold
		if n.Lost {
			extra = append(extra, components.Tag("Lost", "theme.primary"))
		}
	case model.KindLost:
		icon, color = "ghost", "theme.accent"
	case model.KindCarved:
		icon, color = "sparkle", "theme.info"
		title.FontWeight = giode.FontWeightSemiBold
	}
	kids := []giode.Element{icons.Icon(icon, 16, color), giode.Box(giode.Styles{FlexGrow: 1}, giode.Text(n.Title, title))}
	kids = append(kids, extra...)
	return giode.Row(giode.Styles{Width: -1, Height: -1, Gap: theme.S2, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, Padding: giode.Inset{Right: theme.S2}}, kids...)
}

func (r *Results) row(i int) giode.Element {
	rows := r.m.Rows()
	if i >= len(rows) {
		return giode.Box(giode.Styles{})
	}
	e := rows[i]
	compact := giode.Compact()
	p := giode.Keyed("row", e, func() *giode.PressableElement {
		return giode.Pressable(func(giode.PressState) giode.Element { return nil })
	})
	return p.Content(func(s giode.PressState) giode.Element {
		return components.FileRow(e, r.m.IsChecked(e), r.m.Preview == e, compact, s)
	}).OnClick(func(ev giode.ClickEvent) {
		if ev.PositionDp.X < components.CheckZone && ev.Source == giode.ClickPointer {
			r.m.Toggle(e)
			return
		}
		if e.IsDir && (ev.Clicks == 2 || compact) {
			giode.AfterLayout(func() { r.m.OpenEntry(e) })
			return
		}
		r.m.Preview = e
		if compact && !e.IsDir {
			r.previewOpen = true
		}
	}).Label(e.Name)
}

// View lays out the screen.
func (r *Results) View() giode.Element {
	r.syncTree()
	files, size := r.m.Selection()
	r.recover.Disabled(files == 0)
	if files > 0 {
		r.recover.Label(fmt.Sprintf("Recover %d", files))
	} else {
		r.recover.Label("Recover")
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

// summary is the line along the top: what was scanned and what was found.
func (r *Results) summary(compact bool) giode.Element {
	res := r.m.Result
	name := r.m.SourceName()
	files, deleted, carved := 0, 0, 0
	if res != nil {
		files, deleted, carved = res.Stats()
	}
	facts := fmt.Sprintf("%d files · %d deleted · %d by signature", files, deleted, carved)
	if res != nil && res.Duration > 0 {
		facts += " · scanned in " + friendly(res.Duration)
	}
	st := giode.Styles{Width: -1, Gap: theme.S4, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap,
		Padding: giode.SymmetricInset(theme.S5, theme.S3), BorderWidth: 1, BorderColor: "theme.border", Background: "theme.background"}
	left := giode.Stack(giode.Styles{Gap: 2, FlexGrow: 1},
		giode.Text(name, theme.Heading().With(giode.Styles{TextOverflow: giode.TextOverflowEllipsis})),
		giode.Text(facts, theme.Caption().With(giode.Styles{FontWeight: giode.FontWeightNormal})),
	)
	kids := []giode.Element{r.newScan, left}
	if disk.IsGone(r.m.ScanErr) {
		kids = append(kids, components.Tag("Drive disconnected", "theme.danger"))
	} else if r.m.ScanErr != nil {
		kids = append(kids, components.Tag("Stopped early", "theme.warning"))
	}
	if compact {
		st.Padding = giode.SymmetricInset(theme.S3, theme.S2)
	}
	return giode.Row(st, kids...)
}

func (r *Results) sidebar() giode.Element {
	return giode.Stack(giode.Styles{Width: 264, Height: -1, Background: "theme.surface", BorderWidth: 1, BorderColor: "theme.border",
		Padding: giode.Inset{Top: theme.S3, Left: theme.S2, Right: theme.S1}, Gap: theme.S2},
		giode.Box(giode.Styles{Padding: giode.SymmetricInset(theme.S2, theme.S1)}, giode.Text("Found", theme.Overline())),
		giode.Box(giode.Styles{Width: -1, FlexGrow: 1}, r.tree),
	)
}

func (r *Results) table(compact bool) giode.Element {
	bar := []giode.Element{}
	if compact && r.m.Folder != nil && r.m.Folder.Parent != nil {
		bar = append(bar, r.up)
	}
	bar = append(bar, giode.Box(giode.Styles{FlexGrow: 1}, components.Breadcrumb(r.m.Folder, r.m.OpenFolder)))
	if !compact {
		bar = append(bar, r.show, giode.Box(giode.Styles{Width: 200}, r.search))
	}
	var filters []giode.Element
	if compact {
		filters = append(filters, r.show)
	}
	for i, c := range r.cats {
		filters = append(filters, c.View(r.m.Filter.Category == r.catValues[i]))
	}
	var head []giode.Element
	head = append(head,
		giode.Row(giode.Styles{Width: -1, Gap: theme.S3, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap}, bar...),
	)
	if compact {
		// No header to click on a phone: a menu sorts instead.
		for i, c := range model.SortChoices {
			if c.Order == r.m.Order && r.sort.Selected() != i {
				r.sort.SetSelected(i)
			}
		}
		head = append(head, giode.Row(giode.Styles{Width: -1, Gap: theme.S2, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
			giode.Box(giode.Styles{FlexGrow: 1}, r.search),
			giode.Box(giode.Styles{Width: 168}, r.sort)))
	}
	head = append(head, giode.Row(giode.Styles{Width: -1, Gap: theme.S2, Align: giode.AlignCenter}, filters...))

	rows := r.m.Rows()
	var list giode.Element = r.list
	if len(rows) == 0 {
		list = r.empty()
	}
	parts := []giode.Element{
		giode.Stack(giode.Styles{Width: -1, Gap: theme.S3, Padding: giode.Inset{Top: theme.S4, Left: theme.S5, Right: theme.S5, Bottom: theme.S3}}, head...),
	}
	if !compact {
		parts = append(parts, giode.Box(giode.Styles{Width: -1, BorderWidth: 1, BorderColor: theme.Hairline()}, components.FileHeader(r.checkAll, r.m.Order, r.m.SortBy)))
	}
	if r.m.Searching() && len(rows) > 0 {
		parts = append(parts, giode.Box(giode.Styles{Padding: giode.SymmetricInset(theme.S5, 6)},
			giode.Text(count(len(rows), "match", "matches")+" in this folder and the folders inside it", theme.Caption())))
	}
	parts = append(parts, giode.Box(giode.Styles{Width: -1, FlexGrow: 1}, list))
	return giode.Stack(giode.Styles{Width: -1, Height: -1}, parts...)
}

func (r *Results) empty() giode.Element {
	switch {
	case r.m.Folder == nil && r.m.Mode == scan.Quick:
		return components.EmptyState("search", "No file system could be read",
			"The drive may have been formatted or its partitions lost. A deep scan reads every sector, finds lost partitions and recognizes files by their signature.", r.deep)
	case r.m.Folder == nil:
		return components.EmptyState("search", "Nothing was found",
			"Every sector was read, but nothing recognizable is left. The drive may be encrypted (BitLocker, FileVault, LUKS), wiped, or an SSD that has trimmed its free space.", nil)
	case r.m.Searching():
		return components.EmptyState("search", "No matches", "Nothing here matches the search and filters.", nil)
	case r.m.Filter.Show == model.ShowDeleted:
		return components.EmptyState("check", "No deleted files here",
			"This folder has no deleted files. Switch to All files to see everything in it.", nil)
	}
	return components.EmptyState("folder-open", "This folder is empty", "", nil)
}

// bar is the strip along the bottom: the selection and the recover button.
func (r *Results) bar(files int, size int64, compact bool) giode.Element {
	msg := "Check the files and folders to recover"
	if files > 0 {
		msg = count(files, "file", "files") + " selected · " + recovery.FormatBytes(size)
	}
	kids := []giode.Element{giode.Box(giode.Styles{FlexGrow: 1}, giode.Text(msg, theme.Secondary()))}
	if files > 0 {
		kids = append(kids, r.clear)
	}
	kids = append(kids, r.recover)
	pad := giode.SymmetricInset(theme.S5, theme.S3)
	if compact {
		pad = giode.UniformInset(theme.S3)
	}
	return giode.Row(giode.Styles{Width: -1, Gap: theme.S3, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap,
		Padding: pad, Background: "theme.surface", BorderWidth: 1, BorderColor: "theme.border"}, kids...)
}
