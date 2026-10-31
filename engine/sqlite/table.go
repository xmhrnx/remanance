package sqlite

import (
	"fmt"
	"slices"
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

// readSchema reads sqlite_master, which is the table on page 1:
// type, name, tbl_name, rootpage, sql.
func (db *DB) readSchema() error {
	db.tables = map[string]*Table{}
	return db.walk(1, func(_ int64, payload []byte) bool {
		v := db.record(payload)
		if len(v) < 5 {
			return true
		}
		kind, _ := v[0].(string)
		name, _ := v[1].(string)
		root, _ := v[3].(int64)
		sql, _ := v[4].(string)
		if kind != "table" || root <= 0 || strings.Contains(strings.ToUpper(sql), "WITHOUT ROWID") {
			return true
		}
		cols, types, rowid := columns(sql)
		db.tables[strings.ToLower(name)] = &Table{Name: name, Root: uint32(root), Columns: cols, Types: types, rowidCol: rowid, db: db}
		return true
	})
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

// Tables lists the database's tables.
func (db *DB) Tables() ([]*Table, error) {
	if db.tables == nil {
		if err := db.readSchema(); err != nil {
			return nil, err
		}
	}
	out := make([]*Table, 0, len(db.tables))
	for _, t := range db.tables {
		out = append(out, t)
	}
	slices.SortFunc(out, func(a, b *Table) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}

// columns reads the column names and declared types out of a CREATE
// TABLE statement, and which column is an alias of the rowid.
func columns(sql string) (cols, types []string, rowid int) {
	rowid = -1
	open, end := strings.Index(sql, "("), strings.LastIndex(sql, ")")
	if open < 0 || end <= open {
		return nil, nil, -1
	}
	depth, from := 0, open+1
	var defs []string
	for i := open + 1; i < end; i++ {
		switch sql[i] {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				defs = append(defs, sql[from:i])
				from = i + 1
			}
		}
	}
	defs = append(defs, sql[from:end])
	for _, d := range defs {
		f := strings.Fields(d)
		if len(f) == 0 {
			continue
		}
		switch strings.ToUpper(f[0]) {
		case "CONSTRAINT", "PRIMARY", "UNIQUE", "CHECK", "FOREIGN":
			continue
		}
		name := strings.Trim(f[0], "\"`[]'")
		up := strings.ToUpper(strings.Join(f, " "))
		if len(f) > 1 && strings.ToUpper(f[1]) == "INTEGER" && strings.Contains(up, "PRIMARY KEY") && !strings.Contains(up, "DESC") {
			rowid = len(cols)
		}
		cols = append(cols, name)
		types = append(types, declared(f[1:]))
	}
	return cols, types, rowid
}

// declared is the type a column definition gives, the words after its
// name up to the first constraint.
func declared(words []string) string {
	var t []string
	for _, w := range words {
		u := strings.ToUpper(w)
		if i := strings.IndexByte(u, '('); i >= 0 {
			u = u[:i] // VARCHAR(255)
		}
		switch u {
		case "PRIMARY", "NOT", "NULL", "DEFAULT", "UNIQUE", "CHECK", "REFERENCES", "COLLATE", "CONSTRAINT", "GENERATED", "AS":
			return strings.Join(t, " ")
		}
		if u != "" {
			t = append(t, u)
		}
	}
	return strings.Join(t, " ")
}
