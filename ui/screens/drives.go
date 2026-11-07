// Package screens holds DataDoc's pages: the drive list, the scan in
// progress and its results. Each is created once and laid out every
// frame.
package screens

import (
	"errors"
	"strings"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/iconbutton"
	"github.com/mehran9675/giode/components/input"
	"github.com/mehran9675/giode/components/scroll"
	"github.com/mehran9675/giode/components/segmented"
	"github.com/xmhrnx/remanance/engine/disk"
	"github.com/xmhrnx/remanance/engine/iosbackup"
	"github.com/xmhrnx/remanance/engine/scan"
	"github.com/xmhrnx/remanance/locales"
	"github.com/xmhrnx/remanance/ui/components"
	"github.com/xmhrnx/remanance/ui/model"
	"github.com/xmhrnx/remanance/ui/theme"
)

// imageTypes are the disk image files offered when opening one.
var imageTypes = []string{"img", "dd", "raw", "bin", "iso", "dmg", "001", "image"}

// Drives lists the drives and disk images to scan, and starts the scan.
type Drives struct {
	m        *model.Model
	selected string // the chosen drive's ID
	images   []disk.Info
	backups  []disk.Info // backups opened from a folder the user picked
	pickErr  error
	// elevating is set while the system asks for administrator rights.
	elevating bool

	mode  *segmented.Control
	start *giode.ButtonElement
	// password and unlock ask for an encrypted backup's password.
	password *input.Input
	unlock   *giode.ButtonElement
	refresh  *iconbutton.Button
	elevate  *giode.ButtonElement
	view     *scroll.Scroll
}

// NewDrives builds the drive list.
func NewDrives(m *model.Model) *Drives {
	d := &Drives{m: m}
	d.mode = giode.Input.Segmented(locales.T("drives.quick_scan"), locales.T("drives.deep_scan")).Styles(segmentStyles().With(giode.Styles{Width: 280}))
	d.start = components.PrimaryButton(locales.T("drives.start_scan"), "search").OnClick(d.startScan)
	d.refresh = components.IconAction("refresh", locales.T("drives.refresh_the_drive_list")).OnClick(m.LoadDrives)
	d.password = giode.Input.Text(locales.T("drives.backup_password"), giode.Styles{FontSize: 13, Padding: giode.SymmetricInset(10, 8), BorderRadius: theme.Radius}).
		Password(true).Submit(func(string) { d.unlockBackup() })
	d.unlock = components.PrimaryButton(locales.T("drives.unlock"), "lock").OnClick(d.unlockBackup)
	d.elevate = components.SecondaryButton(locales.T("drives.restart_as_administrator"), "shield").OnClick(func() {
		if d.elevating {
			return
		}
		d.elevating = true
		// Off the window's thread: RestartElevated waits for the user to
		// answer the system prompt, and a window that stops handling its
		// messages that long is closed by Windows as hung. It quits this
		// copy once the new one starts.
		go func() {
			err := m.App.RestartElevated()
			m.App.Post(func() {
				d.elevating = false
				if err != nil && !errors.Is(err, giode.ErrDeclined) {
					d.pickErr = err
				}
			})
		}()
	})
	d.view = giode.Scroll(d.content, giode.Styles{ScrollBar: giode.ScrollBar{AutoHide: true}})
	return d
}

func (d *Drives) chosen() (disk.Info, bool) {
	for _, l := range [][]disk.Info{d.images, d.backups, d.m.Drives} {
		for _, i := range l {
			if i.ID == d.selected {
				return i, true
			}
		}
	}
	return disk.Info{}, false
}

func (d *Drives) startScan() {
	info, ok := d.chosen()
	if !ok {
		return
	}
	mode := scan.Quick
	if d.mode.Selected() == 1 {
		mode = scan.Deep
	}
	d.m.Open(info, func() {
		d.m.Within = ""
		d.m.StartScan(mode)
	})
}

// unlockBackup scans the chosen encrypted backup with the password
// typed.
func (d *Drives) unlockBackup() {
	info, ok := d.chosen()
	if !ok || !isBackup(info) || d.password.Text() == "" || d.m.Opening {
		return
	}
	iosbackup.SetPassword(strings.TrimPrefix(info.ID, "iosbackup:"), d.password.Text())
	d.startScan()
}

// needsPassword tells whether err asks for an encrypted backup's
// password.
func needsPassword(err error) bool {
	return errors.Is(err, iosbackup.ErrPassword) || errors.Is(err, iosbackup.ErrWrongPassword)
}

// isBackup tells the iPhone backups apart from drives.
func isBackup(info disk.Info) bool { return strings.HasPrefix(info.ID, "iosbackup:") }

// View lays out the screen.
func (d *Drives) View() giode.Element {
	if _, ok := d.chosen(); !ok {
		d.selected = ""
		// Preselect the first drive that isn't the system's.
		for _, i := range d.m.Drives {
			if !i.System && i.Kind != disk.KindVolume {
				d.selected = i.ID
				break
			}
		}
	}
	_, ok := d.chosen()
	d.start.Disabled(!ok || d.m.Opening)
	if d.m.Opening {
		d.start.Label(locales.T("drives.opening"))
	} else {
		d.start.Label(locales.T("drives.start_scan"))
	}
	return giode.Stack(giode.Styles{Width: -1, Height: -1},
		giode.Box(giode.Styles{Width: -1, FlexGrow: 1}, d.view),
		d.bar(),
	)
}
