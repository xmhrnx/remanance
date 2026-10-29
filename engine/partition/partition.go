// Package partition reads partition tables and searches a device for the
// volumes that a lost, damaged or overwritten table no longer lists.
package partition

// Scheme is the kind of partition table a partition came from.
type Scheme string

const (
	SchemeMBR   Scheme = "MBR"
	SchemeGPT   Scheme = "GPT"
	SchemeNone  Scheme = "None"  // a file system on the whole device
	SchemeFound Scheme = "Found" // found by scanning, not in any table
)

// Partition is a region of a device holding a volume.
type Partition struct {
	Index    int
	Scheme   Scheme
	Offset   int64 // bytes from the start of the device
	Length   int64
	Type     string // human name of the partition type
	TypeID   string // MBR type byte as hex or GPT type GUID
	Name     string // GPT partition name
	FS       string // file system found inside, "" if none recognized
	Label    string // volume label, when the file system has one
	Lost     bool   // not listed in the partition table
	Bootable bool
}

// End is the offset just past the partition.
func (p Partition) End() int64 { return p.Offset + p.Length }
