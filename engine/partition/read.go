package partition

import "io"

// Table is what a device's partition table says.
type Table struct {
	Scheme     Scheme
	Partitions []Partition
	// Damaged is set when the table had to be read from a backup or its
	// checksum didn't match.
	Damaged bool
}

// Read reads the partition table of a device and identifies the file
// system in each partition. A device formatted without a table (a USB
// stick or SD card formatted whole) gets one partition covering it.
func Read(r io.ReaderAt, size int64, sectorSize int) (Table, error) {
	if sectorSize <= 0 {
		sectorSize = 512
	}
	var t Table
	mbr, isMBR, err := ReadMBR(r, size, sectorSize)
	if err != nil {
		return t, err
	}
	protective := isMBR && len(mbr) > 0 && mbr[0].TypeID == "0xEE"
	if gpt, ok, damaged, err := ReadGPT(r, size, sectorSize); ok || protective {
		if err != nil {
			return t, err
		}
		t = Table{Scheme: SchemeGPT, Partitions: gpt, Damaged: damaged || !ok}
	} else if isMBR {
		t = Table{Scheme: SchemeMBR, Partitions: mbr}
	}
	if len(t.Partitions) == 0 {
		if v, ok := Probe(r, 0); ok {
			t = Table{Scheme: SchemeNone, Partitions: []Partition{{
				Index: 1, Scheme: SchemeNone, Offset: 0, Length: size, Type: v.FS, FS: v.FS, Label: v.Label,
			}}}
		}
	}
	for i := range t.Partitions {
		p := &t.Partitions[i]
		if p.FS != "" {
			continue
		}
		if v, ok := Probe(r, p.Offset); ok {
			p.FS, p.Label = v.FS, v.Label
		}
	}
	return t, nil
}
