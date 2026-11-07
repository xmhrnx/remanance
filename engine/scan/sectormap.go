package scan

import "sync/atomic"

// sectorMap tracks the map cells, safe for concurrent use.
type sectorMap struct {
	size  int64
	cells [MapCells]atomic.Uint32
}

func (m *sectorMap) mark(off, n int64, c Cell) {
	if m.size <= 0 || n <= 0 {
		return
	}
	first := off * MapCells / m.size
	last := (off + n - 1) * MapCells / m.size
	for i := max(first, 0); i <= min(last, MapCells-1); i++ {
		for {
			old := m.cells[i].Load()
			// A cell shows the most interesting thing in it.
			if Cell(old) >= c && !(c == CellBad) {
				break
			}
			if m.cells[i].CompareAndSwap(old, uint32(c)) {
				break
			}
		}
	}
}

// cell is the map cell holding byte off.
func (m *sectorMap) cell(off int64) int {
	if m.size <= 0 {
		return -1
	}
	return int(min(max(off*MapCells/m.size, 0), MapCells-1))
}

func (m *sectorMap) snapshot() []Cell {
	out := make([]Cell, MapCells)
	for i := range out {
		out[i] = Cell(m.cells[i].Load())
	}
	return out
}
