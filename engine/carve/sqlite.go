package carve

import "io"

// parseSQLite sizes a database from its header: page size times page
// count. Phones keep messages, call logs and contacts in SQLite.
func parseSQLite(r io.ReaderAt, limit int64) Match {
	h := at(r, 0, 100, limit)
	if h == nil {
		return Match{}
	}
	page := int64(be16(h[16:]))
	if page == 1 {
		page = 65536
	}
	if page < 512 || page&(page-1) != 0 || h[21] != 64 || h[22] != 32 || h[23] != 32 {
		return Match{}
	}
	pages := int64(be32(h[28:]))
	// The page count is trustworthy only when written by the same change
	// as the header (SQLite 3.7.0 and later).
	if pages == 0 || be32(h[24:]) != be32(h[92:]) {
		return Match{Length: min(limit, page*1024), Partial: true}
	}
	if page*pages > limit {
		return Match{Length: limit, Partial: true}
	}
	return Match{Length: page * pages}
}

func init() {
	Register(&Signature{
		Name: "SQLite database", Ext: "sqlite", Category: Databases, MaxSize: 16 * GB,
		Magics: []Magic{{0, []byte("SQLite format 3\x00")}},
		Parse:  parseSQLite,
	})
}
