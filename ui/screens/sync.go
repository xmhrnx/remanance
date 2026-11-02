package screens

import (
	"time"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/copybutton"
	"github.com/mehran9675/giode/components/input"
	"github.com/mehran9675/giode/components/scroll"
	"github.com/mehran9675/giode/components/toggle"
	"github.com/xmhrnx/remanance/engine/devsync"
	"github.com/xmhrnx/remanance/engine/recovery"
	"github.com/xmhrnx/remanance/locales"
	"github.com/xmhrnx/remanance/ui/components"
	"github.com/xmhrnx/remanance/ui/icons"
	"github.com/xmhrnx/remanance/ui/model"
	"github.com/xmhrnx/remanance/ui/theme"
)

// Sync keeps folders the same on the user's own devices, straight from
// one to the other: this device, the devices it knows, requests waiting
// for an answer and the folders it syncs.
type Sync struct {
	m          *model.Model
	scroll     *scroll.Scroll
	name       *input.Input
	background *toggle.Switch
	addDevice  *giode.ButtonElement
	addFolder  *giode.ButtonElement
	syncNow    *giode.ButtonElement
	copyID     *copybutton.CopyButton
	device     *AddDevice
	folder     *SyncFolder
	confirm    *Confirm
}

// NewSync builds the sync screen.
func NewSync(m *model.Model) *Sync {
	s := &Sync{m: m, device: NewAddDevice(m), folder: NewSyncFolder(m), confirm: NewConfirm()}
	s.scroll = giode.Scroll(s.content, giode.Styles{ScrollBar: giode.ScrollBar{AutoHide: true}})
	s.name = giode.Input.Text(locales.T("sync.this_device_s_name"), giode.Styles{FontSize: 14, Padding: giode.SymmetricInset(10, 9), BorderRadius: theme.Radius}).
		Submit(func(v string) {
			if m.Sync.Node != nil && v != "" {
				m.Sync.Node.SetName(v)
			}
		})
	s.background = giode.Input.Switch(locales.T("sync.keep_syncing_while_the_app_is"), giode.Styles{FontSize: 13, Color: "theme.text", Background: "theme.primary"}).
		OnChange(m.SetSyncBackground)
	s.addDevice = components.SecondaryButton(locales.T("sync.add_device"), "plus").OnClick(s.device.Open)
	s.addFolder = components.PrimaryButton(locales.T("sync.sync_a_folder"), "folder").OnClick(func() { s.folder.OpenNew("") })
	s.syncNow = components.GhostButton(locales.T("sync.sync_now"), "refresh").OnClick(func() {
		if m.Sync.Node != nil {
			m.Sync.Node.SyncNow()
		}
	})
	s.copyID = giode.CopyButton(func() string {
		if m.Sync.Node == nil {
			return ""
		}
		return m.Sync.Node.ID()
	}).Labels(locales.T("sync.copy"), locales.T("sync.copied"))
	return s
}

// View lays out the screen, starting syncing the first time.
func (s *Sync) View() giode.Element {
	s.m.StartSync()
	if n := s.m.Sync.Node; n != nil && s.name.Text() == "" {
		s.name.SetText(n.Config().Name)
	}
	s.background.SetChecked(s.m.Sync.Background)
	return giode.Stack(giode.Styles{Width: -1, Height: -1},
		giode.Box(giode.Styles{Width: -1, FlexGrow: 1}, s.scroll),
		s.device.View(), s.folder.View(), s.confirm.View())
}

func (s *Sync) content() giode.Element {
	compact := giode.Compact()
	st := &s.m.Sync
	kids := []giode.Element{
		toolHeader("sync", locales.T("sync.sync_your_devices"),
			locales.T("sync.keep_folders_the_same_on_your")),
	}
	if st.Err != nil {
		kids = append(kids, components.Banner("alert", "theme.danger", locales.T("sync.couldnt_start", "error", st.Err.Error()), nil))
	}
	if st.Node == nil {
		return page(compact, kids...)
	}
	kids = append(kids, s.thisDevice())
	if p := st.Status.Pending; len(p) > 0 {
		kids = append(kids, s.pending(p))
	}
	kids = append(kids,
		section(locales.T("sync.folders"), append(s.folders(), giode.Row(giode.Styles{Gap: theme.S2, Align: giode.AlignCenter}, s.addFolder, s.syncNow))...),
		section(locales.T("sync.devices"), append(s.devices(), giode.Row(giode.Styles{Gap: theme.S2}, s.addDevice))...),
		s.background,
	)
	return page(compact, kids...)
}

// thisDevice is the card with this device's name and the ID the others
// add it by.
func (s *Sync) thisDevice() giode.Element {
	id := s.m.Sync.Node.ID()
	return components.Card(giode.Styles{Width: -1},
		giode.Row(giode.Styles{Width: -1, Gap: theme.S3, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
			icons.Icon("devices", 20, "theme.primary"),
			giode.Text(locales.T("sync.this_device"), theme.Heading()),
		),
		s.name,
		giode.Text(locales.T("sync.its_id_the_other_devices_add"), theme.Caption()),
		giode.Row(giode.Styles{Width: -1, Gap: theme.S2, Align: giode.AlignCenter},
			giode.Box(giode.Styles{FlexGrow: 1, Padding: giode.SymmetricInset(theme.S3, 10), Background: theme.Raised(), BorderRadius: theme.Radius},
				giode.Text(id, theme.Code().With(giode.Styles{FontSize: 13}))),
			s.copyID,
		),
	)
}

// pending lists devices asking to sync and folders offered.
func (s *Sync) pending(ps []devsync.Pending) giode.Element {
	var rows []giode.Element
	for _, p := range ps {
		p := p
		key := p.Device + "/" + p.Folder
		dismiss := giode.Keyed("sync-dismiss", key, func() *giode.ButtonElement { return components.GhostButton(locales.T("sync.ignore"), "") }).
			OnClick(func() {
				if s.m.Sync.Node != nil {
					s.m.Sync.Node.Dismiss(p)
					s.m.RefreshSync()
				}
			})
		var icon, title, sub string
		var accept *giode.ButtonElement
		if p.Folder == "" {
			icon, title, sub = "devices", locales.T("sync.wants_to_connect", "name", p.Name), p.Device
			accept = giode.Keyed("sync-accept", key, func() *giode.ButtonElement { return components.PrimaryButton(locales.T("sync.add_device"), "") }).
				OnClick(func() { s.device.addID(p.Device, p.Name, p.Addr) })
		} else {
			icon, title, sub = "folder", locales.T("sync.shares", "name", p.Name, "folder", p.Label), locales.T("sync.accept_it_to_choose_where_it")
			accept = giode.Keyed("sync-accept", key, func() *giode.ButtonElement { return components.PrimaryButton(locales.T("sync.accept"), "") }).
				OnClick(func() { s.folder.OpenOffer(p) })
		}
		rows = append(rows, components.Card(giode.Styles{Width: -1, FlexDirection: giode.FlexDirectionRow, Align: giode.AlignCenter, Wrap: giode.WrapWrap,
			BorderColor: giode.Mix("theme.primary", "theme.border", 0.5)},
			icons.Icon(icon, 20, "theme.primary"),
			giode.Stack(giode.Styles{FlexGrow: 1, Gap: 2, MinWidth: 200},
				giode.Text(title, theme.Body().With(giode.Styles{FontWeight: giode.FontWeightSemiBold})),
				giode.Text(sub, theme.Caption())),
			dismiss, accept,
		))
	}
	return section(locales.T("sync.waiting_for_you"), rows...)
}

// devices lists the devices this one knows.
func (s *Sync) devices() []giode.Element {
	ds := s.m.Sync.Status.Devices
	if len(ds) == 0 {
		return []giode.Element{giode.Text(locales.T("sync.no_devices_yet_add_your_other"), theme.Secondary().With(giode.Styles{FontSize: 13}))}
	}
	var rows []giode.Element
	for _, d := range ds {
		d := d
		remove := giode.Keyed("sync-device-remove", d.ID, func() *giode.ButtonElement { return components.GhostButton(locales.T("sync.remove"), "") }).
			OnClick(func() {
				s.confirm.Ask(locales.T("sync.remove_device", "name", d.Name), locales.T("sync.it_stops_syncing_with_this_device"), locales.T("sync.remove"), func() {
					if s.m.Sync.Node != nil {
						s.m.Sync.Node.RemoveDevice(d.ID)
						s.m.RefreshSync()
					}
				})
			})
		dot, state := "theme.muted", locales.T("sync.not_connected")
		if d.Connected {
			dot, state = "theme.success", locales.T("sync.connected")
			if d.Addr != "" {
				state = locales.T("sync.connected_from", "addr", d.Addr)
			}
		}
		rows = append(rows, giode.Row(giode.Styles{Width: -1, Gap: theme.S3, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap,
			Padding: giode.SymmetricInset(theme.S4, theme.S3), Background: "theme.surface", BorderWidth: 1, BorderColor: "theme.border", BorderRadius: theme.Radius},
			icons.Icon("devices", 18, "theme.text"),
			giode.Stack(giode.Styles{FlexGrow: 1, Gap: 2},
				giode.Text(d.Name, theme.Body().With(giode.Styles{FontWeight: giode.FontWeightSemiBold, MaxLines: 1})),
				giode.Row(giode.Styles{Gap: 6, Align: giode.AlignCenter},
					giode.Box(giode.Styles{Width: 7, Height: 7, BorderRadius: 4, Background: dot}),
					giode.Text(state, theme.Caption()))),
			remove,
		))
	}
	return rows
}

// folders lists the folders this device syncs, each with how it goes.
func (s *Sync) folders() []giode.Element {
	fs := s.m.Sync.Status.Folders
	if len(fs) == 0 {
		return []giode.Element{giode.Text(locales.T("sync.no_folders_yet_pick_one_to"), theme.Secondary().With(giode.Styles{FontSize: 13}))}
	}
	var rows []giode.Element
	for _, f := range fs {
		rows = append(rows, s.folderCard(f))
	}
	return rows
}

func (s *Sync) folderCard(f devsync.FolderStatus) giode.Element {
	color := map[devsync.FolderState]string{
		devsync.StateIdle: "theme.success", devsync.StateScanning: "theme.primary", devsync.StateSyncing: "theme.primary",
		devsync.StatePaused: "theme.muted", devsync.StateError: "theme.danger", devsync.StateWaiting: "theme.warning",
	}[f.State]
	pauseLabel, pauseIcon := locales.T("sync.pause"), "pause"
	if f.Paused {
		pauseLabel, pauseIcon = locales.T("sync.resume"), "play"
	}
	pause := giode.Keyed("sync-pause", f.ID, func() *giode.ButtonElement { return components.GhostButton(pauseLabel, pauseIcon) })
	pause.Label(pauseLabel)
	pause.OnClick(func() {
		if s.m.Sync.Node != nil {
			nf := f.Folder
			nf.Paused = !nf.Paused
			s.m.Sync.Node.UpdateFolder(nf)
			s.m.RefreshSync()
		}
	})
	remove := giode.Keyed("sync-folder-remove", f.ID, func() *giode.ButtonElement { return components.GhostButton(locales.T("sync.stop_syncing"), "") }).
		OnClick(func() {
			s.confirm.Ask(locales.T("sync.stop_syncing_folder", "folder", f.Label), locales.T("sync.the_folder_and_its_files_stay"), locales.T("sync.stop_syncing"), func() {
				if s.m.Sync.Node != nil {
					s.m.Sync.Node.RemoveFolder(f.ID)
					s.m.RefreshSync()
				}
			})
		})

	detail := locales.T("sync.folder_detail", "files", count(f.Files, "file"), "size", recovery.FormatBytes(f.Bytes), "mode", label(devsync.ModeLabels[f.Mode]))
	shared := count(len(f.Devices), "device")
	if len(f.Devices) > 0 {
		shared = locales.T("sync.online", "online", f.Online, "devices", shared)
	}
	kids := []giode.Element{
		giode.Row(giode.Styles{Width: -1, Gap: theme.S3, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
			icons.Icon("folder", 22, "theme.primary"),
			giode.Stack(giode.Styles{FlexGrow: 1, Gap: 2},
				giode.Text(f.Label, theme.Body().With(giode.Styles{FontSize: 15, FontWeight: giode.FontWeightSemiBold, MaxLines: 1})),
				giode.Text(f.Path, theme.Caption().With(giode.Styles{MaxLines: 1}))),
			giode.Row(giode.Styles{Gap: 6, Align: giode.AlignCenter, Padding: giode.SymmetricInset(10, 4), BorderRadius: 12,
				Background: giode.Mix(color, "theme.surface", 0.14)},
				giode.Box(giode.Styles{Width: 7, Height: 7, BorderRadius: 4, Background: color}),
				giode.Text(label(f.State.Label()), theme.Caption().With(giode.Styles{Color: "theme.text", FontWeight: giode.FontWeightSemiBold}))),
		),
	}
	if f.State == devsync.StateSyncing && f.Need > 0 {
		var done float32
		if total := f.Bytes + f.NeedBytes; total > 0 {
			done = float32(f.Bytes) / float32(total)
		}
		kids = append(kids, giode.Progress(done, giode.Styles{Width: -1, Height: 6, BorderRadius: 3, Background: theme.Raised(), Color: "theme.primary"}),
			giode.Text(locales.T("sync.to_go", "files", count(f.Need, "file"), "size", recovery.FormatBytes(f.NeedBytes)), theme.Caption()))
	}
	if f.Err != "" {
		kids = append(kids, giode.Text(f.Err, theme.Caption().With(giode.Styles{Color: "theme.danger"})))
	}
	last := locales.T("sync.not_synced_yet")
	if !f.LastSync.IsZero() {
		last = locales.T("sync.synced_ago", "when", ago(f.LastSync))
	}
	kids = append(kids, giode.Row(giode.Styles{Width: -1, Gap: theme.S2, Align: giode.AlignCenter, Wrap: giode.WrapWrap},
		giode.Stack(giode.Styles{FlexGrow: 1, Gap: 2, MinWidth: 200},
			giode.Text(detail, theme.Caption()),
			giode.Text(shared+" · "+last, theme.Caption())),
		pause, remove,
	))
	return components.Card(giode.Styles{Width: -1}, kids...)
}

// ago says how long ago t was, roughly.
func ago(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return locales.T("sync.just_now")
	case d < time.Hour:
		return locales.T("sync.ago", "time", count(int(d/time.Minute), "minute"))
	case d < 24*time.Hour:
		return locales.T("sync.ago", "time", count(int(d/time.Hour), "hour"))
	}
	return t.Format("2 Jan 15:04")
}
