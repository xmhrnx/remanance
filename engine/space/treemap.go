package space

// Rect is a tile of a treemap, in the units of the area laid out.
type Rect struct {
	X, Y, W, H float64
}

// Tile is where one node goes.
type Tile struct {
	Node *Node
	Rect
}

// Layout lays nodes, largest first, out in area as a squarified treemap:
// each tile's area is in proportion to its size, and the tiles are kept
// as close to square as the sizes allow, so small ones stay visible and
// easy to point at. Nodes of size 0 are left out.
func Layout(nodes []*Node, area Rect) []Tile {
	var items []*Node
	var sum float64
	for _, n := range nodes {
		if n.Size > 0 {
			items = append(items, n)
			sum += float64(n.Size)
		}
	}
	if sum == 0 || area.W <= 0 || area.H <= 0 {
		return nil
	}
	scale := area.W * area.H / sum
	out := make([]Tile, 0, len(items))
	for len(items) > 0 {
		short := min(area.W, area.H)
		// Grow the row while it makes its tiles more square.
		n := 1
		rowSum := float64(items[0].Size) * scale
		best := worst(items[:1], rowSum, short, scale)
		for n < len(items) {
			s := rowSum + float64(items[n].Size)*scale
			w := worst(items[:n+1], s, short, scale)
			if w > best {
				break
			}
			n, rowSum, best = n+1, s, w
		}
		row := items[:n]
		items = items[n:]
		// The row runs along the area's short side.
		thick := rowSum / short
		pos := 0.0
		for _, it := range row {
			length := float64(it.Size) * scale / thick
			t := Tile{Node: it}
			if area.W >= area.H {
				t.Rect = Rect{X: area.X, Y: area.Y + pos, W: thick, H: length}
			} else {
				t.Rect = Rect{X: area.X + pos, Y: area.Y, W: length, H: thick}
			}
			pos += length
			out = append(out, t)
		}
		if area.W >= area.H {
			area.X += thick
			area.W -= thick
		} else {
			area.Y += thick
			area.H -= thick
		}
		if area.W <= 0 || area.H <= 0 {
			break
		}
	}
	return out
}

// worst is the worst aspect ratio of a row of items taking area s along
// a side short long.
func worst(row []*Node, s, short, scale float64) float64 {
	hi := float64(row[0].Size) * scale
	lo := float64(row[len(row)-1].Size) * scale
	s2, w2 := s*s, short*short
	return max(w2*hi/s2, s2/(w2*lo))
}
