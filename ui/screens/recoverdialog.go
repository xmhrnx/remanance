package screens

import (
	"context"
	"os"
	"path/filepath"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/checkbox"
	"github.com/mehran9675/giode/components/dialog"
	"github.com/mehran9675/giode/components/input"
	"github.com/xmhrnx/remanance/locales"
	"github.com/xmhrnx/remanance/ui/components"
	"github.com/xmhrnx/remanance/ui/model"
	"github.com/xmhrnx/remanance/ui/platform"
	"github.com/xmhrnx/remanance/ui/theme"
)

// RecoverDialog asks where to save the checked files, copies them and
// says how it went.
type RecoverDialog struct {
	m    *model.Model
	open bool
	dlg  *dialog.Dialog

	dest    *input.Input
	choose  *giode.ButtonElement
	folders *checkbox.Checkbox
	start   *giode.ButtonElement
	cancel  *giode.ButtonElement
	stop    *giode.ButtonElement
	reveal  *giode.ButtonElement
	done    *giode.ButtonElement
}

// NewRecoverDialog builds the dialog, closed.
func NewRecoverDialog(m *model.Model) *RecoverDialog {
	d := &RecoverDialog{m: m}
	d.dlg = giode.Dialog(&d.open, d.content, giode.Styles{Width: -1, MaxWidth: 520, BorderRadius: theme.RadiusLarge, Padding: giode.UniformInset(theme.S6),
		Background: "theme.surface", BorderWidth: 1, BorderColor: "theme.border"})
	d.dest = giode.Input.Text(locales.T("recoverdialog.folder_to_save_into"), giode.Styles{FontSize: 13, Padding: giode.SymmetricInset(10, 9), BorderRadius: theme.Radius})
	d.dest.SetText(platform.DefaultDestination())
	d.choose = components.SecondaryButton(locales.T("recoverdialog.choose"), "folder").OnClick(func() {
		opts := giode.FolderOptions{Title: locales.T("recoverdialog.where_should_the_recovered_files_go"), Directory: d.dest.Text()}
		go func() {
			if dir, err := m.App.PickFolder(context.Background(), opts); err == nil {
				m.App.Post(func() { d.dest.SetText(dir) })
			}
		}()
	})
	d.folders = giode.Input.Checkbox(locales.T("recoverdialog.keep_the_folders_the_files_were"), giode.Styles{FontSize: 13, Color: "theme.text", Background: "theme.primary", BorderColor: "theme.border"})
	d.folders.SetChecked(true)
	d.start = components.PrimaryButton(locales.T("recoverdialog.recover"), "recover").OnClick(d.begin)
	d.cancel = components.GhostButton(locales.T("recoverdialog.cancel"), "").OnClick(func() { d.open = false })
	d.stop = components.SecondaryButton(locales.T("recoverdialog.stop"), "stop").OnClick(m.CancelRecover)
	d.reveal = components.SecondaryButton(locales.T("recoverdialog.show_the_files"), "external").OnClick(d.show)
	d.done = components.PrimaryButton(locales.T("recoverdialog.done"), "check").OnClick(func() {
		d.open = false
		m.Report, m.RecoverErr = nil, nil
	})
	return d
}

// Open shows the dialog for the checked files.
func (d *RecoverDialog) Open() {
	if !d.m.Recovering {
		d.m.Report, d.m.RecoverErr = nil, nil
	}
	d.open = true
}

func (d *RecoverDialog) begin() {
	dir := d.dest.Text()
	if platform.ChooseDestination && d.m.CheckDestination(dir) != nil {
		return
	}
	sink, err := platform.Sink(dir)
	if err != nil {
		d.m.RecoverErr = err
		return
	}
	d.m.StartRecover(sink, d.folders.Checked())
}

func (d *RecoverDialog) show() {
	if d.m.Report == nil {
		return
	}
	target := filepath.Join(d.m.Report.Location, "Recovery report.txt")
	if _, err := os.Stat(target); err != nil {
		target = d.m.Report.Location
	}
	go d.m.App.Share(context.Background(), giode.ShareFile{Path: target})
}

// View places the dialog; it takes no space while closed.
func (d *RecoverDialog) View() giode.Element {
	// It stays open while files are being copied.
	d.dlg.Dismissible(!d.m.Recovering)
	return d.dlg
}

func (d *RecoverDialog) content() giode.Element {
	switch {
	case d.m.Recovering:
		return d.running()
	case d.m.Report != nil:
		return d.finished()
	}
	return d.setup()
}
