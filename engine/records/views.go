package records

import (
	"bytes"
	"encoding/binary"
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

// cells reads the columns of a row found by name.
type cells struct {
	db      *sqlite.DB
	t       *sqlite.Table
	r       sqlite.Record
	lookups map[string]map[int64]string
}

func (c cells) get(col string) any {
	i := c.t.Col(col)
	if i < 0 || i >= len(c.r.Values) {
		return nil
	}
	return c.r.Values[i]
}

func (c cells) str(col string) string { return format(c.get(col)) }

func (c cells) int(col string) int64 {
	v, _ := c.get(col).(int64)
	return v
}

// first is the first of the columns that has something in it.
func (c cells) first(cols ...string) string {
	for _, col := range cols {
		if s := c.str(col); s != "" {
			return s
		}
	}
	return ""
}

// lookup finds, in another table still in the database, the value of
// col in the row whose key column is key: a contact's number by its
// handle.
func (c cells) lookup(table, keyCol, col string, key int64) string {
	id := table + "." + keyCol + "." + col
	m, ok := c.lookups[id]
	if !ok {
		m = map[int64]string{}
		if t, err := c.db.Table(table); err == nil {
			k, v := t.Col(keyCol), t.Col(col)
			if k >= 0 && v >= 0 {
				t.Rows(func(_ int64, row []any) bool {
					if n, ok := row[k].(int64); ok {
						m[n] = format(row[v])
					}
					return true
				})
			}
		}
		c.lookups[id] = m
	}
	return m[key]
}

func pick(b bool, yes, no string) string {
	if b {
		return yes
	}
	return no
}

var iosMessages = &app{app: "Messages", views: []view{{
	table: "message", file: "Messages.csv",
	head: []string{"Date", "Contact", "Sent or received", "Service", "Text"},
	row: func(c cells) []string {
		text := c.str("text")
		if text == "" {
			// iOS 16 and later keep the text in attributedBody only.
			if b, ok := c.get("attributedBody").([]byte); ok {
				text = typedText(b)
			}
		}
		return []string{
			appleDate(c.get("date")),
			c.lookup("handle", "ROWID", "id", c.int("handle_id")),
			pick(c.int("is_from_me") == 1, "Sent", "Received"),
			c.str("service"),
			text,
		}
	},
}, {
	table: "handle", file: "Contacts messaged.csv",
	head: []string{"Number or address", "Service"},
	row:  func(c cells) []string { return []string{c.str("id"), c.str("service")} },
}, {
	table: "attachment", file: "Attachments.csv",
	head: []string{"Date", "File", "Type", "Name"},
	row: func(c cells) []string {
		return []string{appleDate(c.get("created_date")), c.str("filename"), c.str("mime_type"), c.str("transfer_name")}
	},
}}}

var iosCalls = &app{app: "Calls", views: []view{{
	table: "ZCALLRECORD", file: "Calls.csv",
	head: []string{"Date", "Number", "Name", "Direction", "Answered", "Seconds"},
	row: func(c cells) []string {
		return []string{
			appleDate(c.get("ZDATE")),
			c.str("ZADDRESS"),
			c.str("ZNAME"),
			pick(c.int("ZORIGINATED") == 1, "Outgoing", "Incoming"),
			pick(c.int("ZANSWERED") == 1, "Yes", "No"),
			c.str("ZDURATION"),
		}
	},
}, {
	table: "call", file: "Calls.csv",
	head: []string{"Date", "Number", "Seconds", "Flags"},
	row: func(c cells) []string {
		return []string{unixMillis(c.get("date")), c.str("address"), c.str("duration"), c.str("flags")}
	},
}}}

var iosContacts = &app{app: "Contacts", views: []view{{
	table: "ABPerson", file: "Contacts.csv",
	head: []string{"First name", "Last name", "Company", "Note", "Created"},
	row: func(c cells) []string {
		return []string{c.str("First"), c.str("Last"), c.str("Organization"), c.str("Note"), appleDate(c.get("CreationDate"))}
	},
}, {
	table: "ABMultiValue", file: "Phone numbers and emails.csv",
	head: []string{"Contact", "Value"},
	row: func(c cells) []string {
		id := c.int("record_id")
		name := strings.TrimSpace(c.lookup("ABPerson", "ROWID", "First", id) + " " + c.lookup("ABPerson", "ROWID", "Last", id))
		return []string{name, c.str("value")}
	},
}}}

var iosNotes = &app{app: "Notes", views: []view{{
	table: "ZICCLOUDSYNCINGOBJECT", file: "Notes.csv",
	head: []string{"Modified", "Title", "Beginning"},
	row: func(c cells) []string {
		return []string{appleDate(c.get("ZMODIFICATIONDATE1")), c.first("ZTITLE1", "ZTITLE"), c.str("ZSNIPPET")}
	},
}, {
	table: "ZNOTEBODY", file: "Notes.csv",
	head: []string{"Note"},
	row:  func(c cells) []string { return []string{c.str("ZCONTENT")} },
}, {
	table: "ZNOTE", file: "Note titles.csv",
	head: []string{"Modified", "Title", "Summary"},
	row: func(c cells) []string {
		return []string{appleDate(c.get("ZMODIFICATIONDATE")), c.str("ZTITLE"), c.str("ZSUMMARY")}
	},
}}}

var safari = &app{app: "Safari", views: []view{{
	table: "history_items", file: "Pages visited.csv",
	head: []string{"Address", "Visits"},
	row:  func(c cells) []string { return []string{c.str("url"), c.str("visit_count")} },
}, {
	table: "history_visits", file: "Visits.csv",
	head: []string{"Date", "Title", "Address"},
	row: func(c cells) []string {
		return []string{appleDate(c.get("visit_time")), c.str("title"), c.lookup("history_items", "id", "url", c.int("history_item"))}
	},
}}}

var whatsAppIOS = &app{app: "WhatsApp", views: []view{{
	table: "ZWAMESSAGE", file: "Messages.csv",
	head: []string{"Date", "From", "To", "Sent or received", "Text"},
	row: func(c cells) []string {
		return []string{
			appleDate(c.get("ZMESSAGEDATE")), c.str("ZFROMJID"), c.str("ZTOJID"),
			pick(c.int("ZISFROMME") == 1, "Sent", "Received"), c.str("ZTEXT"),
		}
	},
}}}

var androidMessages = &app{app: "Messages", views: []view{{
	table: "sms", file: "Messages.csv",
	head: []string{"Date", "Number", "Sent or received", "Text"},
	row: func(c cells) []string {
		dir := "Received"
		switch c.int("type") {
		case 2, 4, 5, 6:
			dir = "Sent"
		case 3:
			dir = "Draft"
		}
		return []string{unixMillis(c.get("date")), c.str("address"), dir, c.str("body")}
	},
}, {
	table: "part", file: "Picture message parts.csv",
	head: []string{"Type", "Name", "Text", "File"},
	row: func(c cells) []string {
		return []string{c.str("ct"), c.first("name", "cl"), c.str("text"), c.str("_data")}
	},
}}}

var androidContacts = &app{app: "Contacts and calls", views: []view{{
	table: "calls", file: "Calls.csv",
	head: []string{"Date", "Number", "Name", "Direction", "Seconds"},
	row: func(c cells) []string {
		dir := map[int64]string{1: "Incoming", 2: "Outgoing", 3: "Missed", 4: "Voicemail", 5: "Rejected", 6: "Blocked"}[c.int("type")]
		return []string{unixMillis(c.get("date")), c.str("number"), c.str("name"), dir, c.str("duration")}
	},
}, {
	table: "raw_contacts", file: "Contacts.csv",
	head: []string{"Name", "Account"},
	row: func(c cells) []string {
		return []string{c.first("display_name", "display_name_alt"), c.str("account_name")}
	},
}, {
	table: "data", file: "Contact details.csv",
	head: []string{"Value", "More"},
	row:  func(c cells) []string { return []string{c.str("data1"), c.str("data2")} },
}}}

var whatsAppAndroid = &app{app: "WhatsApp", views: []view{{
	table: "message", file: "Messages.csv",
	head: []string{"Date", "Chat", "Sent or received", "Text"},
	row: func(c cells) []string {
		chat := c.lookup("chat", "_id", "jid_row_id", c.int("chat_row_id"))
		var jid string
		if n, ok := parseInt(chat); ok {
			jid = c.lookup("jid", "_id", "raw_string", n)
		}
		return []string{unixMillis(c.get("timestamp")), jid, pick(c.int("from_me") == 1, "Sent", "Received"), c.str("text_data")}
	},
}, {
	table: "messages", file: "Messages.csv",
	head: []string{"Date", "Chat", "Sent or received", "Text"},
	row: func(c cells) []string {
		return []string{unixMillis(c.get("timestamp")), c.str("key_remote_jid"), pick(c.int("key_from_me") == 1, "Sent", "Received"), c.str("data")}
	},
}}}

func parseInt(s string) (int64, bool) {
	var n int64
	if s == "" {
		return 0, false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int64(r-'0')
	}
	return n, true
}

// typedText reads the text out of an NSAttributedString archived as a
// typedstream, as iOS keeps a message's in attributedBody: the C string
// after the class name NSString.
func typedText(b []byte) string {
	i := bytes.Index(b, []byte("NSString"))
	if i < 0 {
		return ""
	}
	b = b[i+len("NSString"):]
	j := bytes.IndexByte(b, '+')
	if j < 0 || j > 8 || j+2 > len(b) {
		return ""
	}
	b = b[j+1:]
	n := int(b[0])
	b = b[1:]
	switch {
	case n == 0x81 && len(b) >= 2:
		n = int(binary.LittleEndian.Uint16(b))
		b = b[2:]
	case n == 0x82 && len(b) >= 4:
		n = int(binary.LittleEndian.Uint32(b))
		b = b[4:]
	}
	return string(b[:min(n, len(b))])
}
