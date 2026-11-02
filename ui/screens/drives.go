// Package screens holds DataDoc's pages: the drive list, the scan in
// progress and its results. Each is created once and laid out every
// frame.
package screens

import (
	"context"
	"errors"
	"slices"
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
	"github.com/xmhrnx/remanance/ui/icons"
	"github.com/xmhrnx/remanance/ui/model"
	"github.com/xmhrnx/remanance/ui/platform"
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
	d.m.Open(info, func() { d.m.StartScan(mode) })
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

func (d *Drives) pickImage() {
	go func() {
		files, err := d.m.App.PickFiles(context.Background(), giode.PickOptions{Types: imageTypes})
		if err != nil || len(files) == 0 {
			return
		}
		f := files[0]
		d.m.App.Post(func() {
			defer f.Close()
			if f.Path == "" {
				d.pickErr = errNoPath
				return
			}
			dev, err := disk.OpenImage(f.Path)
			if err != nil {
				d.pickErr = err
				return
			}
			info := dev.Info()
			dev.Close()
			d.addImage(info)
		})
	}()
}

// pickBackup opens an iPhone backup from a folder the user picks: the
// backup itself, or the Backup folder of a copy kept elsewhere.
func (d *Drives) pickBackup() {
	opts := giode.FolderOptions{Title: locales.T("drives.choose_an_iphone_or_ipad_backup")}
	go func() {
		dir, err := d.m.App.PickFolder(context.Background(), opts)
		if err != nil || dir == "" {
			return
		}
		b, err := iosbackup.ReadPicked(dir)
		d.m.App.Post(func() {
			if err != nil {
				d.pickErr = err
				return
			}
			info := iosbackup.Info(b)
			d.selected, d.pickErr = info.ID, nil
			for _, i := range slices.Concat(d.backups, d.m.Drives) {
				if i.ID == info.ID {
					return
				}
			}
			d.backups = append(d.backups, info)
		})
	}()
}

// isBackup tells the iPhone backups apart from drives.
func isBackup(info disk.Info) bool { return strings.HasPrefix(info.ID, "iosbackup:") }

func (d *Drives) addImage(info disk.Info) {
	for _, i := range d.images {
		if i.ID == info.ID {
			d.selected = info.ID
			return
		}
	}
	d.images = append(d.images, info)
	d.selected, d.pickErr = info.ID, nil
}

// View lays out the screen.
func (d *Drives) View() giode.Element {
	if _, ok := d.chosen(); !ok {
		d.selected = ""
		// Preselect the first drive that isn't the system's.
		for _, i := range d.m.Drives {
			if !i.System {
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

func (d *Drives) content() giode.Element {
	compact := giode.Compact()
	pad := theme.S8
	if compact {
		pad = theme.S4
	}
	var notes []giode.Element
	if d.m.NeedsAdmin && !giode.IsElevated() {
		d.elevate.Disabled(d.elevating)
		if d.elevating {
			d.elevate.Label(locales.T("drives.waiting_for_permission"))
		} else {
			d.elevate.Label(locales.T("drives.restart_as_administrator"))
		}
		notes = append(notes, components.Banner("lock", "theme.warning",
			locales.T("drives.reading_drives_directly_needs_administrator_rights"), d.elevate))
	}
	if needsPassword(d.m.OpenErr) {
		d.unlock.Disabled(d.m.Opening)
		if d.m.Opening {
			d.unlock.Label(locales.T("drives.unlocking"))
		} else {
			d.unlock.Label(locales.T("drives.unlock"))
		}
		notes = append(notes, components.Banner("lock", "theme.warning", d.m.OpenErr.Error(),
			giode.Row(giode.Styles{Gap: theme.S2, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
				giode.Box(giode.Styles{Width: 220}, d.password), d.unlock)))
	}
	for _, err := range []error{d.m.OpenErr, d.m.DrivesErr, d.pickErr} {
		if err != nil && !(d.m.NeedsAdmin && err == d.m.OpenErr) && !needsPassword(err) {
			notes = append(notes, components.Banner("alert", "theme.danger", err.Error(), nil))
		}
	}
	return giode.Box(giode.Styles{Width: -1, Justify: giode.JustifyCenter},
		giode.Stack(giode.Styles{Width: -1, MaxWidth: 1080, Padding: giode.UniformInset(pad), Gap: theme.S6},
			giode.Stack(giode.Styles{Gap: theme.S2},
				giode.Text(locales.T("drives.choose_where_to_look"), theme.Hero()),
				giode.Box(giode.Styles{MaxWidth: 620}, giode.Text(
					locales.T("drives.datadoc_reads_the_drive_without_changing"),
					theme.Secondary().With(giode.Styles{FontSize: 14, LineHeight: 21}))),
			),
			giode.Stack(giode.Styles{Gap: theme.S3}, notes...),
			d.section(locales.T("drives.drives"), d.refresh, d.driveCards(compact)),
			d.section(locales.T("drives.disk_images"), nil, d.imageCards(compact)),
			d.backupSection(compact),
		),
	)
}

func (d *Drives) section(title string, action giode.Element, body giode.Element) giode.Element {
	head := []giode.Element{giode.Text(title, theme.Overline().With(giode.Styles{FontSize: 11}))}
	if action != nil {
		head = append(head, action)
	}
	return giode.Stack(giode.Styles{Width: -1, Gap: theme.S3},
		giode.Row(giode.Styles{Gap: theme.S2, Align: giode.AlignCenter, Height: 32}, head...),
		body,
	)
}

func (d *Drives) cardWidth(compact bool) giode.Styles {
	if compact {
		return giode.Styles{Width: -1, Height: 112}
	}
	return giode.Styles{Width: 328, Height: 112}
}

func (d *Drives) card(info disk.Info, compact bool) giode.Element {
	p := giode.Keyed("drive", info.ID, func() *giode.PressableElement {
		return giode.Pressable(func(giode.PressState) giode.Element { return nil })
	})
	p.Content(func(s giode.PressState) giode.Element {
		return components.DriveCard(info, info.ID == d.selected, s)
	}).OnClick(func(e giode.ClickEvent) {
		d.selected = info.ID
		if e.Clicks == 2 {
			d.startScan()
		}
	}).Label(info.Name)
	return giode.Box(d.cardWidth(compact), p)
}

func (d *Drives) driveCards(compact bool) giode.Element {
	if d.m.DrivesLoading && len(d.m.Drives) == 0 {
		return giode.Row(giode.Styles{Gap: theme.S3, Align: giode.AlignCenter, Height: 112},
			giode.Spinner(giode.Styles{Width: 18, Height: 18, Color: "theme.primary"}),
			giode.Text(locales.T("drives.looking_for_drives"), theme.Secondary()))
	}
	if len(d.m.Drives) == 0 {
		return components.EmptyState("drive", locales.T("drives.no_drives_found"),
			locales.T("drives.connect_the_drive_you_want_to"), nil)
	}
	var cards []giode.Element
	for _, info := range d.m.Drives {
		if !isBackup(info) {
			cards = append(cards, d.card(info, compact))
		}
	}
	return giode.Row(giode.Styles{Width: -1, Gap: theme.S3}, cards...)
}

// backupSection lists the iPhone and iPad backups on this computer, on
// desktops, which hold photos and app data the phone has since lost.
func (d *Drives) backupSection(compact bool) giode.Element {
	if !platform.ReadsBackups {
		return giode.Box(giode.Styles{})
	}
	var cards []giode.Element
	for _, info := range slices.Concat(d.m.Drives, d.backups) {
		if isBackup(info) {
			cards = append(cards, d.card(info, compact))
		}
	}
	cards = append(cards, d.openCard("open-backup", "phone", locales.T("drives.open_an_iphone_backup"), locales.T("drives.a_backup_folder_made_by_finder"), compact, d.pickBackup))
	return d.section(locales.T("drives.iphone_and_ipad_backups"), nil, giode.Row(giode.Styles{Width: -1, Gap: theme.S3}, cards...))
}

func (d *Drives) imageCards(compact bool) giode.Element {
	var cards []giode.Element
	for _, info := range d.images {
		cards = append(cards, d.card(info, compact))
	}
	cards = append(cards, d.openCard("open-image", "plus", locales.T("drives.open_a_disk_image"), locales.T("drives.a_copy_of_a_drive_img"), compact, d.pickImage))
	return giode.Row(giode.Styles{Width: -1, Gap: theme.S3}, cards...)
}

// openCard is the card that opens something not listed: a disk image, a
// backup kept elsewhere.
func (d *Drives) openCard(key, icon, title, detail string, compact bool, open func()) giode.Element {
	p := giode.Keyed("drive", key, func() *giode.PressableElement {
		return giode.Pressable(func(giode.PressState) giode.Element { return nil })
	})
	p.Content(func(s giode.PressState) giode.Element {
		border := "theme.border"
		if s.Hovered {
			border = "theme.primary"
		}
		return giode.Row(giode.Styles{Width: -1, Height: -1, Gap: theme.S3, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap,
			Padding: giode.UniformInset(theme.S4), BorderRadius: theme.RadiusLarge, BorderWidth: 1, BorderColor: border,
			Cursor: giode.CursorPointer},
			giode.Box(giode.Styles{Width: 44, Height: 44, BorderRadius: 12, Align: giode.AlignCenter, Justify: giode.JustifyCenter,
				Background: giode.Mix("theme.primary", "theme.surface", 0.12)}, icons.Icon(icon, 20, "theme.primary")),
			giode.Stack(giode.Styles{Gap: 3, FlexGrow: 1},
				giode.Text(title, theme.Heading()),
				giode.Text(detail, theme.Secondary()),
			),
		)
	}).OnClick(func(giode.ClickEvent) { open() }).Label(title)
	return giode.Box(d.cardWidth(compact), p)
}

// bar is the strip along the bottom: the scan mode and the start button.
func (d *Drives) bar() giode.Element {
	hint := locales.T("drives.reads_the_file_systems_for_deleted")
	if d.mode.Selected() == 1 {
		hint = locales.T("drives.also_reads_every_sector_finds_lost")
	}
	compact := giode.Compact()
	inner := []giode.Element{
		giode.Stack(giode.Styles{Gap: 6, FlexGrow: 1},
			d.mode,
			giode.Text(hint, theme.Caption().With(giode.Styles{FontWeight: giode.FontWeightNormal})),
		),
		d.start,
	}
	st := giode.Styles{Width: -1, Gap: theme.S4, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap,
		Padding: giode.SymmetricInset(theme.S8, theme.S4), Background: "theme.surface",
		BorderWidth: 1, BorderColor: "theme.border"}
	if compact {
		st.Padding = giode.UniformInset(theme.S4)
		st.Wrap = giode.WrapWrap
	}
	return giode.Row(st, inner...)
}

// AddImage lists a disk image, as when one is opened with the app, and
// selects it.
func (d *Drives) AddImage(path string) error {
	dev, err := disk.OpenImage(path)
	if err != nil {
		return err
	}
	info := dev.Info()
	dev.Close()
	d.addImage(info)
	return nil
}
