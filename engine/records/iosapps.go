package records

import "strings"

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
