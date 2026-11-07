package records

import "github.com/xmhrnx/remanance/engine/sqlite"

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
