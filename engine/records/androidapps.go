package records

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
