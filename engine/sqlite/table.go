package sqlite

import (
	"fmt"
	"strings"
)

// Table is a table of the database.
type Table struct {
	Name    string
	Root    uint32
	Columns []string
	// Types are the columns' declared types, upper case: "INTEGER",
	// "TEXT", "" when the column has none.
	Types []string
	// rowidCol is the INTEGER PRIMARY KEY column, which the file stores
	// as NULL and keeps as the rowid; -1 if there is none.
	rowidCol int
	db       *DB
}

// Table finds a table by name, ignoring case.
func (db *DB) Table(name string) (*Table, error) {
	if db.tables == nil {
		if err := db.readSchema(); err != nil {
			return nil, err
		}
	}
	t, ok := db.tables[strings.ToLower(name)]
	if !ok {
		return nil, fmt.Errorf("sqlite: no table %s", name)
	}
	return t, nil
}

// Col is the index of a column, ignoring case; -1 if there's none.
func (t *Table) Col(name string) int {
	for i, c := range t.Columns {
		if strings.EqualFold(c, name) {
			return i
		}
	}
	return -1
}

// Rows calls fn with every row, its values in column order, until fn
// returns false. A row written before columns were added is short of
// them; they read as nil.
func (t *Table) Rows(fn func(rowid int64, row []any) bool) error {
	return t.db.walk(t.Root, func(rowid int64, payload []byte) bool {
		v := t.db.record(payload)
		row := make([]any, max(len(t.Columns), len(v)))
		copy(row, v)
		if t.rowidCol >= 0 && t.rowidCol < len(row) && row[t.rowidCol] == nil {
			row[t.rowidCol] = rowid
		}
		return fn(rowid, row)
	})
}
