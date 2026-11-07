package records

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"unicode/utf8"

	"github.com/xmhrnx/remanance/engine/sqlite"
)

// plain lays out a table's rows found as they are, with a column for
// their rowid and one for where each was found.
func plain(t *sqlite.Table, rows []sqlite.Record) ([]string, [][]string) {
	head := append([]string{"rowid"}, t.Columns...)
	head = append(head, "Found in")
	var body [][]string
	for _, r := range rows {
		line := []string{rowid(r)}
		for i := range t.Columns {
			var v any
			if i < len(r.Values) {
				v = r.Values[i]
			}
			line = append(line, format(v))
		}
		body = append(body, append(line, r.Where))
	}
	return head, body
}

func rowid(r sqlite.Record) string {
	if r.Rowid == 0 {
		return ""
	}
	return fmt.Sprint(r.Rowid)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// format writes a value for a CSV cell; data that isn't text as hex.
func format(v any) string {
	switch v := v.(type) {
	case nil:
		return ""
	case string:
		return v
	case []byte:
		if utf8.Valid(v) && printable(string(v)) {
			return string(v)
		}
		if len(v) > 64 {
			return fmt.Sprintf("x'%x…' (%d bytes)", v[:64], len(v))
		}
		return fmt.Sprintf("x'%x'", v)
	case float64:
		return fmt.Sprintf("%g", v)
	}
	return fmt.Sprint(v)
}

func printable(s string) bool {
	for _, r := range s {
		if r < 0x20 && r != '\n' && r != '\r' && r != '\t' {
			return false
		}
	}
	return true
}

// bom is UTF-8's byte order mark.
const bom = "\xef\xbb\xbf"

// writeCSV writes rows as CSV with a byte order mark, which Excel needs
// to read UTF-8.
func writeCSV(head []string, body [][]string) []byte {
	var b bytes.Buffer
	b.WriteString(bom)
	w := csv.NewWriter(&b)
	w.Write(head)
	w.WriteAll(body)
	return b.Bytes()
}
