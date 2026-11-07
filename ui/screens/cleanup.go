package screens

import (
	"strings"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/scroll"
	"github.com/xmhrnx/remanance/engine/cleanup"
	"github.com/xmhrnx/remanance/engine/recovery"
	"github.com/xmhrnx/remanance/engine/trash"
	"github.com/xmhrnx/remanance/locales"
	"github.com/xmhrnx/remanance/ui/components"
	"github.com/xmhrnx/remanance/ui/model"
)

// maxCleanRows is how many of a group's items are listed when it's
// opened; the rest are checked and unchecked with the group.
const maxCleanRows = 50

// cleanKinds say what each kind of file is, and its icon.
var cleanKinds = map[cleanup.Kind]struct{ icon, about string }{
	cleanup.Temp:         {"clock", locales.T("cleanup.about_temp")},
	cleanup.Caches:       {"layers", locales.T("cleanup.about_caches")},
	cleanup.BuildCaches:  {"box", locales.T("cleanup.about_build_caches")},
	cleanup.Logs:         {"document", locales.T("cleanup.about_logs")},
	cleanup.Trash:        {"trash", locales.T("cleanup.about_trash")},
	cleanup.Installers:   {"archive", locales.T("cleanup.about_installers")},
	cleanup.OldDownloads: {"history", locales.T("cleanup.about_old_downloads")},
}

// Cleanup frees space: it finds temporary files, caches, logs, the
// trash and forgotten downloads, and cleans the ones checked.
type Cleanup struct {
	m       *model.Model
	scroll  *scroll.Scroll
	open    map[cleanup.Kind]bool // groups showing their items
	again   *giode.ButtonElement
	stop    *giode.ButtonElement
	clean   *giode.ButtonElement
	confirm *Confirm
}

// NewCleanup builds the clean up screen.
func NewCleanup(m *model.Model) *Cleanup {
	c := &Cleanup{m: m, open: map[cleanup.Kind]bool{}, confirm: NewConfirm()}
	c.scroll = giode.Scroll(c.content, giode.Styles{ScrollBar: giode.ScrollBar{AutoHide: true}})
	c.again = components.GhostButton(locales.T("cleanup.look_again"), "refresh").OnClick(m.FindCleanup)
	c.stop = components.SecondaryButton(locales.T("cleanup.stop"), "stop").OnClick(m.CancelCleanup)
	c.clean = components.PrimaryButton("", "broom").OnClick(c.askClean)
	return c
}

// askClean says what cleaning the checked items will do, and does it if
// the user agrees.
func (c *Cleanup) askClean() {
	items, size := c.m.CleanChecked()
	if len(items) == 0 {
		return
	}
	var deletes, empties, trashes bool
	for _, it := range items {
		switch {
		case it.Kind == cleanup.Trash:
			empties = true
		case it.Kind.ToTrash():
			trashes = true
		default:
			deletes = true
		}
	}
	var msg []string
	if deletes {
		msg = append(msg, locales.T("cleanup.confirm_delete"))
	}
	if empties {
		msg = append(msg, locales.T("cleanup.confirm_empty"))
	}
	if trashes && trash.Supported {
		msg = append(msg, locales.T("cleanup.confirm_trash"))
	} else if trashes {
		msg = append(msg, locales.T("cleanup.confirm_downloads_deleted"))
	}
	c.confirm.Ask(locales.T("cleanup.confirm_title", "size", recovery.FormatBytes(size)), strings.Join(msg, " "), locales.T("cleanup.clean_up"), c.m.CleanUp)
}

// View lays out the screen, looking the first time it's shown.
func (c *Cleanup) View() giode.Element {
	st := &c.m.Clean
	if !st.Looked {
		c.m.FindCleanup()
	}
	kids := []giode.Element{giode.Box(giode.Styles{Width: -1, FlexGrow: 1}, c.scroll)}
	if !st.Looking && len(st.Groups) > 0 {
		kids = append(kids, c.foot())
	}
	return giode.Stack(giode.Styles{Width: -1, Height: -1}, append(kids, c.confirm.View())...)
}
