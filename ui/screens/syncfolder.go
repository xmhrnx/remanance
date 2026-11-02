package screens

import (
	"context"
	"path/filepath"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/checkbox"
	"github.com/mehran9675/giode/components/dialog"
	"github.com/mehran9675/giode/components/input"
	"github.com/mehran9675/giode/components/segmented"
	"github.com/mehran9675/giode/components/toggle"
	"github.com/xmhrnx/remanance/engine/devsync"
	"github.com/xmhrnx/remanance/locales"
	"github.com/xmhrnx/remanance/ui/components"
	"github.com/xmhrnx/remanance/ui/icons"
	"github.com/xmhrnx/remanance/ui/model"
	"github.com/xmhrnx/remanance/ui/platform"
	"github.com/xmhrnx/remanance/ui/theme"
)

// SyncFolder adds a folder to sync, or accepts one another device
// offers: where it goes, who it's shared with and how it syncs.
type SyncFolder struct {
	m      *model.Model
	open   bool
	dlg    *dialog.Dialog
	offer  *devsync.Pending // set when accepting
	path   *input.Input
	label  *input.Input
	browse *giode.ButtonElement
	mode   *segmented.Control
	keep   *toggle.Switch
	devs   map[string]*checkbox.Checkbox
	ok     *giode.ButtonElement
	cancel *giode.ButtonElement
	err    string
}

// NewSyncFolder builds the dialog, closed.
func NewSyncFolder(m *model.Model) *SyncFolder {
	d := &SyncFolder{m: m, devs: map[string]*checkbox.Checkbox{}}
	d.dlg = giode.Dialog(&d.open, d.content, giode.Styles{Width: 560, BorderRadius: theme.RadiusLarge, Padding: giode.UniformInset(theme.S6),
		Background: "theme.surface", BorderWidth: 1, BorderColor: "theme.border"})
	field := func(p string) *input.Input {
		return giode.Input.Text(p, giode.Styles{FontSize: 13, Padding: giode.SymmetricInset(10, 9), BorderRadius: theme.Radius})
	}
	d.path = field(locales.T("syncfolder.the_folder_on_this_device"))
	d.label = field(locales.T("syncfolder.its_name_as_the_other_devices"))
	d.browse = components.SecondaryButton(locales.T("syncfolder.choose"), "folder").OnClick(func() {
		go func() {
			dir, err := m.App.PickFolder(context.Background(), giode.FolderOptions{Directory: d.path.Text()})
			if err == nil {
				m.App.Post(func() {
					d.path.SetText(dir)
					if d.label.Text() == "" {
						d.label.SetText(filepath.Base(dir))
					}
				})
			}
		}()
	})
	d.mode = giode.Input.Segmented(labels(devsync.ModeLabels)...).Styles(segmentStyles().With(giode.Styles{Width: -1, MaxWidth: 420, FontSize: 12.5})).
		OnChange(func(i int) { d.keep.SetChecked(i == int(devsync.ReceiveOnly)) })
	d.keep = giode.Input.Switch(locales.T("syncfolder.keep_what_changes_replace_in_datadoc"), giode.Styles{FontSize: 13, Color: "theme.text", Background: "theme.primary"})
	d.ok = components.PrimaryButton(locales.T("syncfolder.sync_this_folder"), "sync").OnClick(d.submit)
	d.cancel = components.GhostButton(locales.T("syncfolder.cancel"), "").OnClick(func() { d.open = false })
	return d
}

// OpenNew shows the dialog for a folder of this device's, starting at
// path if given.
func (d *SyncFolder) OpenNew(path string) {
	d.offer, d.err = nil, ""
	d.path.SetText(path)
	d.label.SetText("")
	if path != "" {
		d.label.SetText(filepath.Base(path))
	}
	d.mode.SetSelected(0)
	d.keep.SetChecked(false)
	d.devs = map[string]*checkbox.Checkbox{}
	d.ok.Label(locales.T("syncfolder.sync_this_folder"))
	d.open = true
}

// OpenOffer shows the dialog to accept a folder another device offers.
func (d *SyncFolder) OpenOffer(p devsync.Pending) {
	d.offer, d.err = &p, ""
	d.path.SetText(model.SuggestSyncPath(p.Label))
	d.label.SetText(p.Label)
	d.mode.SetSelected(0)
	d.keep.SetChecked(false)
	d.ok.Label(locales.T("syncfolder.accept"))
	d.open = true
}

func (d *SyncFolder) submit() {
	path := d.path.Text()
	if path == "" {
		d.err = locales.T("syncfolder.choose_where_the_folder_goes_on")
		return
	}
	mode := devsync.Mode(d.mode.Selected())
	var err error
	if d.offer != nil {
		err = d.m.AcceptSyncFolder(*d.offer, path, mode)
	} else {
		f := devsync.Folder{Label: d.label.Text(), Path: path, Mode: mode, Versions: d.keep.Checked()}
		for id, cb := range d.devs {
			if cb.Checked() {
				f.Devices = append(f.Devices, id)
			}
		}
		err = d.m.AddSyncFolder(f)
	}
	if err != nil {
		d.err = err.Error()
		return
	}
	d.open = false
}

// View places the dialog; it takes no space while closed.
func (d *SyncFolder) View() giode.Element { return d.dlg }

func (d *SyncFolder) content() giode.Element {
	title, sub := locales.T("syncfolder.sync_a_folder"), locales.T("syncfolder.its_files_are_kept_the_same")
	if d.offer != nil {
		title, sub = locales.T("syncfolder.accept_folder", "folder", d.offer.Label), locales.T("syncfolder.offered_by", "name", d.offer.Name)
	}
	where := []giode.Element{giode.Box(giode.Styles{FlexGrow: 1}, d.path)}
	if platform.PicksFolders {
		where = append(where, d.browse)
	}
	kids := []giode.Element{
		giode.Row(giode.Styles{Gap: theme.S3, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
			giode.Box(giode.Styles{Width: 40, Height: 40, BorderRadius: 12, Align: giode.AlignCenter, Justify: giode.JustifyCenter,
				Background: giode.Mix("theme.primary", "theme.surface", 0.14)}, icons.Icon("sync", 20, "theme.primary")),
			giode.Stack(giode.Styles{FlexGrow: 1, Gap: 2}, giode.Text(title, theme.Heading()), giode.Text(sub, theme.Caption()))),
		section(locales.T("syncfolder.folder"), giode.Row(giode.Styles{Width: -1, Gap: theme.S2, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap}, where...)),
	}
	if d.offer == nil {
		kids = append(kids, section(locales.T("syncfolder.name"), d.label))
		var boxes []giode.Element
		for _, dev := range d.m.Sync.Status.Devices {
			cb, ok := d.devs[dev.ID]
			if !ok {
				cb = giode.Input.Checkbox(dev.Name, giode.Styles{FontSize: 13, Color: "theme.text", Background: "theme.primary", BorderColor: "theme.border"})
				cb.SetChecked(true)
				d.devs[dev.ID] = cb
			}
			boxes = append(boxes, cb)
		}
		if len(boxes) == 0 {
			boxes = append(boxes, giode.Text(locales.T("syncfolder.no_devices_added_yet_the_folder"), theme.Caption()))
		}
		kids = append(kids, section(locales.T("syncfolder.share_with"), boxes...))
	}
	modeNote := map[int]string{
		0: locales.T("syncfolder.changes_on_any_device_reach_the"),
		1: locales.T("syncfolder.changes_here_go_to_the_others"),
		2: locales.T("syncfolder.changes_from_the_others_come_here"),
	}[d.mode.Selected()]
	kids = append(kids, section(locales.T("syncfolder.how_it_syncs"), d.mode, giode.Text(modeNote, theme.Caption()), d.keep))
	if d.err != "" {
		kids = append(kids, giode.Text(d.err, theme.Caption().With(giode.Styles{Color: "theme.danger"})))
	}
	kids = append(kids, giode.Row(giode.Styles{Width: -1, Gap: theme.S2, Justify: giode.JustifyEnd}, d.cancel, d.ok))
	return giode.Stack(giode.Styles{Width: -1, Gap: theme.S4}, kids...)
}
