package sqlite

import "encoding/binary"

// trunkPage marks a free list trunk page in freelist's map.
const trunkPage = -1

// freelist maps the pages on the free list: trunkPage for the trunk
// pages, which list the others, and 1 for those.
func (db *DB) freelist() map[uint32]int {
	out := map[uint32]int{}
	p1, err := db.page(1)
	if err != nil {
		return out
	}
	trunk := binary.BigEndian.Uint32(p1[32:])
	for i := 0; trunk != 0 && trunk <= db.pages && i < int(db.pages); i++ {
		if _, seen := out[trunk]; seen {
			break
		}
		out[trunk] = trunkPage
		p, err := db.page(trunk)
		if err != nil {
			break
		}
		n := int(binary.BigEndian.Uint32(p[4:]))
		for j := 0; j < n && 8+4*j+4 <= len(p); j++ {
			if leaf := binary.BigEndian.Uint32(p[8+4*j:]); leaf != 0 && leaf <= db.pages {
				out[leaf] = 1
			}
		}
		trunk = binary.BigEndian.Uint32(p)
	}
	return out
}
