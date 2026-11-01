package screens

import (
	"fmt"
	"time"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/input"
	"github.com/mehran9675/giode/components/scroll"
	"github.com/mehran9675/giode/components/toggle"
	"github.com/xmhrnx/remanance/engine/devsync"
	"github.com/xmhrnx/remanance/engine/recovery"
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
	device     *AddDevice
	folder     *SyncFolder
	confirm    *Confirm
}

// NewSync builds the sync screen.
func NewSync(m *model.Model) *Sync {
	s := &Sync{m: m, device: NewAddDevice(m), folder: NewSyncFolder(m), confirm: NewConfirm()}
	s.scroll = giode.Scroll(s.content, giode.Styles{ScrollBar: giode.ScrollBar{AutoHide: true}})
	s.name = giode.Input.Text("This device's name", giode.Styles{FontSize: 14, Padding: giode.SymmetricInset(10, 9), BorderRadius: theme.Radius}).
		Submit(func(v string) {
			if m.Sync.Node != nil && v != "" {
				m.Sync.Node.SetName(v)
			}
		})
	s.background = giode.Input.Switch("Keep syncing while the app is out of sight", giode.Styles{FontSize: 13, Color: "theme.text", Background: "theme.primary"}).
		OnChange(m.SetSyncBackground)
	s.addDevice = components.SecondaryButton("Add device", "plus").OnClick(s.device.Open)
	s.addFolder = components.PrimaryButton("Sync a folder", "folder").OnClick(func() { s.folder.OpenNew("") })
	s.syncNow = components.GhostButton("Sync now", "refresh").OnClick(func() {
		if m.Sync.Node != nil {
			m.Sync.Node.SyncNow()
		}
	})
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
		toolHeader("sync", "Sync your devices",
			"Keep folders the same on your computers and phones. Files go straight from one device to the other, encrypted, over your network or the internet: no cloud, no account."),
	}
	if st.Err != nil {
		kids = append(kids, components.Banner("alert", "theme.danger", "Syncing couldn't start: "+st.Err.Error(), nil))
	}
	if st.Node == nil {
		return page(compact, kids...)
	}
	kids = append(kids, s.thisDevice())
	if p := st.Status.Pending; len(p) > 0 {
		kids = append(kids, s.pending(p))
	}
	kids = append(kids,
		section("Folders", append(s.folders(), giode.Row(giode.Styles{Gap: theme.S2, Align: giode.AlignCenter}, s.addFolder, s.syncNow))...),
		section("Devices", append(s.devices(), giode.Row(giode.Styles{Gap: theme.S2}, s.addDevice))...),
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
			giode.Text("This device", theme.Heading()),
		),
		s.name,
		giode.Text("Its ID: the other devices add this one by it, or find it on the same network by themselves.", theme.Caption()),
		giode.Row(giode.Styles{Width: -1, Gap: theme.S2, Align: giode.AlignCenter},
			giode.Box(giode.Styles{FlexGrow: 1, Padding: giode.SymmetricInset(theme.S3, 10), Background: theme.Raised(), BorderRadius: theme.Radius},
				giode.Text(id, theme.Code().With(giode.Styles{FontSize: 13}))),
			giode.CopyButton(func() string { return id }),
		),
	)
}

// pending lists devices asking to sync and folders offered.
func (s *Sync) pending(ps []devsync.Pending) giode.Element {
	var rows []giode.Element
	for _, p := range ps {
		p := p
		key := p.Device + "/" + p.Folder
		dismiss := giode.Keyed("sync-dismiss", key, func() *giode.ButtonElement { return components.GhostButton("Ignore", "") }).
			OnClick(func() {
				if s.m.Sync.Node != nil {
					s.m.Sync.Node.Dismiss(p)
					s.m.RefreshSync()
				}
			})
		var icon, title, sub string
		var accept *giode.ButtonElement
		if p.Folder == "" {
			icon, title, sub = "devices", p.Name+" wants to connect", p.Device
			accept = giode.Keyed("sync-accept", key, func() *giode.ButtonElement { return components.PrimaryButton("Add device", "") }).
				OnClick(func() { s.device.addID(p.Device, p.Name, p.Addr) })
		} else {
			icon, title, sub = "folder", p.Name+" shares “"+p.Label+"”", "Accept it to choose where it goes here."
			accept = giode.Keyed("sync-accept", key, func() *giode.ButtonElement { return components.PrimaryButton("Accept…", "") }).
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
	return section("Waiting for you", rows...)
}

// devices lists the devices this one knows.
func (s *Sync) devices() []giode.Element {
	ds := s.m.Sync.Status.Devices
	if len(ds) == 0 {
		return []giode.Element{giode.Text("No devices yet. Add your other computer or phone to start.", theme.Secondary().With(giode.Styles{FontSize: 13}))}
	}
	var rows []giode.Element
	for _, d := range ds {
		d := d
		remove := giode.Keyed("sync-device-remove", d.ID, func() *giode.ButtonElement { return components.GhostButton("Remove", "") }).
			OnClick(func() {
				s.confirm.Ask("Remove "+d.Name+"?", "It stops syncing with this device. Files already synced stay where they are.", "Remove", func() {
					if s.m.Sync.Node != nil {
						s.m.Sync.Node.RemoveDevice(d.ID)
						s.m.RefreshSync()
					}
				})
			})
		dot, state := "theme.muted", "Not connected"
		if d.Connected {
			dot, state = "theme.success", "Connected"
			if d.Addr != "" {
				state += " · " + d.Addr
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
		return []giode.Element{giode.Text("No folders yet. Pick one to keep the same on your devices.", theme.Secondary().With(giode.Styles{FontSize: 13}))}
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
	pauseLabel, pauseIcon := "Pause", "pause"
	if f.Paused {
		pauseLabel, pauseIcon = "Resume", "play"
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
	remove := giode.Keyed("sync-folder-remove", f.ID, func() *giode.ButtonElement { return components.GhostButton("Stop syncing", "") }).
		OnClick(func() {
			s.confirm.Ask("Stop syncing “"+f.Label+"”?", "The folder and its files stay on this device; changes just stop going between devices.", "Stop syncing", func() {
				if s.m.Sync.Node != nil {
					s.m.Sync.Node.RemoveFolder(f.ID)
					s.m.RefreshSync()
				}
			})
		})

	detail := fmt.Sprintf("%s · %s · %s", count(f.Files, "file", "files"), recovery.FormatBytes(f.Bytes), devsync.ModeLabels[f.Mode])
	shared := count(len(f.Devices), "device", "devices")
	if len(f.Devices) > 0 {
		shared = fmt.Sprintf("%d of %s online", f.Online, shared)
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
				giode.Text(f.State.Label(), theme.Caption().With(giode.Styles{Color: "theme.text", FontWeight: giode.FontWeightSemiBold}))),
		),
	}
	if f.State == devsync.StateSyncing && f.Need > 0 {
		var done float32
		if total := f.Bytes + f.NeedBytes; total > 0 {
			done = float32(f.Bytes) / float32(total)
		}
		kids = append(kids, giode.Progress(done, giode.Styles{Width: -1, Height: 6, BorderRadius: 3, Background: theme.Raised(), Color: "theme.primary"}),
			giode.Text(fmt.Sprintf("%s to go, %s", count(f.Need, "file", "files"), recovery.FormatBytes(f.NeedBytes)), theme.Caption()))
	}
	if f.Err != "" {
		kids = append(kids, giode.Text(f.Err, theme.Caption().With(giode.Styles{Color: "theme.danger"})))
	}
	last := "Not synced yet"
	if !f.LastSync.IsZero() {
		last = "Synced " + ago(f.LastSync)
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
		return "just now"
	case d < time.Hour:
		return count(int(d/time.Minute), "minute", "minutes") + " ago"
	case d < 24*time.Hour:
		return count(int(d/time.Hour), "hour", "hours") + " ago"
	}
	return t.Format("2 Jan 15:04")
}
