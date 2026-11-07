package records

import (
	"strings"

	"github.com/xmhrnx/remanance/engine/fsys"
	"github.com/xmhrnx/remanance/engine/sqlite"
)

// app is a database Find knows: whose it is, and how to lay out the
// tables people want back.
type app struct {
	app   string
	views []view
}

// view lays out one table's rows found, under names people know.
type view struct {
	table string
	file  string
	head  []string
	row   func(c cells) []string
}

// known finds what Find knows of a database, from its name and where it
// is; nil if nothing.
func known(e *fsys.Entry) *app {
	name := strings.ToLower(e.Name)
	p := strings.ToLower(e.Path())
	switch {
	case name == "sms.db":
		return iosMessages
	case name == "call_history.db" || name == "callhistory.storedata":
		return iosCalls
	case name == "addressbook.sqlitedb":
		return iosContacts
	case name == "notestore.sqlite" || name == "notes.sqlite":
		return iosNotes
	case name == "history.db" && strings.Contains(p, "safari"):
		return safari
	case name == "chatstorage.sqlite":
		return whatsAppIOS
	case name == "mmssms.db":
		return androidMessages
	case name == "calllog.db" || name == "contacts2.db":
		return androidContacts
	case name == "msgstore.db":
		return whatsAppAndroid
	}
	return nil
}

func (a *app) find(table string) *view {
	if a == nil {
		return nil
	}
	for i := range a.views {
		if strings.EqualFold(a.views[i].table, table) {
			return &a.views[i]
		}
	}
	return nil
}

// render lays out rows with the view's columns, and where each was
// found.
func (v *view) render(db *sqlite.DB, t *sqlite.Table, rows []sqlite.Record) ([]string, [][]string) {
	head := append(append([]string(nil), v.head...), "Found in")
	lookups := map[string]map[int64]string{}
	var body [][]string
	for _, r := range rows {
		body = append(body, append(v.row(cells{db: db, t: t, r: r, lookups: lookups}), r.Where))
	}
	return head, body
}
