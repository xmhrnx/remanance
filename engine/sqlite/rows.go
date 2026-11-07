package sqlite

import (
	"fmt"
	"strings"
)

// walkInto walks a table b-tree like walk, adding its pages to seen.
func (db *DB) walkInto(root uint32, seen map[uint32]bool, fn func(int64, []byte) bool) error {
	_, err := db.walkPage(root, seen, 0, fn)
	return err
}

// withRowid puts the rowid in the column that aliases it, which the
// file stores as NULL.
func (t *Table) withRowid(rowid int64, v []any) []any {
	row := make([]any, max(len(t.Columns), len(v)))
	copy(row, v)
	if rowid != 0 && t.rowidCol >= 0 && t.rowidCol < len(row) && row[t.rowidCol] == nil {
		row[t.rowidCol] = rowid
	}
	return row
}

// rowKeys are what tell a row found apart from the rows still in the
// table: all its values, and, for an older version of a row since
// changed, its text with its rowid, or its text alone when there's
// enough of it to be unlikely twice.
func rowKeys(t *Table, rowid int64, row []any) []string {
	var all, text strings.Builder
	n := 0
	fmt.Fprintf(&all, "%s\x00", t.Name)
	for i, v := range row {
		if i == t.rowidCol {
			continue
		}
		fmt.Fprintf(&all, "%T:%v\x00", v, v)
		switch v := v.(type) {
		case string:
			text.WriteString(v)
			n += len(v)
		case []byte:
			text.Write(v)
			n += len(v)
		}
		text.WriteByte(0)
	}
	keys := []string{all.String()}
	if n > 0 && rowid != 0 {
		keys = append(keys, fmt.Sprintf("%s\x00%d\x00%s", t.Name, rowid, text.String()))
	}
	if n >= 16 {
		keys = append(keys, t.Name+"\x00text\x00"+text.String())
	}
	return keys
}
