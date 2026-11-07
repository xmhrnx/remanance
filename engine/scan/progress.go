package scan

import "time"

// Phase is the step a scan is on.
type Phase string

const (
	PhaseTable      Phase = "Reading the partition table"
	PhaseFileSystem Phase = "Reading file systems"
	PhaseSectors    Phase = "Searching every sector"
	PhaseLost       Phase = "Reading lost partitions"
	PhaseRecords    Phase = "Looking for deleted messages and records"
	PhaseDone       Phase = "Done"
)

// Progress is a snapshot of a running scan.
type Progress struct {
	Phase    Phase
	Fraction float64 // 0 to 1, over the whole scan
	// Position and Total are the bytes of the sector search.
	Position, Total int64
	Speed           float64 // bytes per second in the sector search
	Remaining       time.Duration
	Files           int // files found through file systems
	Deleted         int // of which deleted or orphaned
	Carved          int // files found by signature
	Partitions      int // volumes found, lost ones included
	BadBytes        int64
	Map             []Cell // the sector map, MapCells long
	// Current is the map cell the sector search is reading, or -1
	// outside it.
	Current int
}

// Cell is one cell of the sector map the UI draws.
type Cell uint8

const (
	CellUnread Cell = iota
	CellRead
	CellMetadata // a file system's structures were read here
	CellFound    // a file was found by signature here
	CellBad      // unreadable sectors
)

// MapCells is how many cells the sector map has.
const MapCells = 1600
