package model

import (
	"strings"

	"github.com/xmhrnx/remanance/engine/fsys"
)

// maxSearchRows caps a search, which walks every folder below.
const maxSearchRows = 50000

// Rows lists what the table shows: the current folder's folders and
// files, or with a query, every match below it.
func (m *Model) Rows() []*fsys.Entry {
	key := rowKey{m.Folder, m.Filter, m.Order}
	if key == m.rowKey && m.rowCache != nil {
		return m.rowCache
	}
	m.rowKey = key
	m.rowCache = m.rowCache[:0]
	if m.Folder == nil {
		return m.rowCache
	}
	f := m.Filter
	f.Query = strings.ToLower(strings.TrimSpace(f.Query))
	if f.Query != "" || f.Category != CatAny {
		m.Folder.Entry.Walk(func(e *fsys.Entry) bool {
			if e != m.Folder.Entry && !e.IsDir && f.keep(e) && len(m.rowCache) < maxSearchRows {
				m.rowCache = append(m.rowCache, e)
			}
			return len(m.rowCache) < maxSearchRows
		})
		m.Order.sortRows(m.rowCache)
		return m.rowCache
	}
	for _, c := range m.Folder.Entry.Children {
		if f.keep(c) {
			m.rowCache = append(m.rowCache, c)
		}
	}
	m.Order.sortRows(m.rowCache)
	return m.rowCache
}

// Searching reports whether the table lists matches from every folder
// below, rather than the folder's own contents.
func (m *Model) Searching() bool {
	return strings.TrimSpace(m.Filter.Query) != "" || m.Filter.Category != CatAny
}

// OpenFolder shows a folder from the table or the tree.
func (m *Model) OpenFolder(n *Node) {
	m.Folder, m.Preview = n, nil
}

// OpenEntry opens a folder listed in the table.
func (m *Model) OpenEntry(e *fsys.Entry) {
	if m.Folder == nil || !e.IsDir {
		return
	}
	if c := m.Folder.Child(e); c != nil {
		m.OpenFolder(c)
	}
}
