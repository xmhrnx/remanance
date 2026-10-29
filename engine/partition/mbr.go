package partition

import (
	"encoding/binary"
	"fmt"
	"io"
)

var mbrTypes = map[byte]string{
	0x01: "FAT12", 0x04: "FAT16", 0x05: "Extended", 0x06: "FAT16", 0x07: "NTFS/exFAT",
	0x0B: "FAT32", 0x0C: "FAT32 (LBA)", 0x0E: "FAT16 (LBA)", 0x0F: "Extended (LBA)",
	0x11: "Hidden FAT12", 0x14: "Hidden FAT16", 0x16: "Hidden FAT16", 0x17: "Hidden NTFS",
	0x1B: "Hidden FAT32", 0x1C: "Hidden FAT32", 0x27: "Windows RE", 0x82: "Linux swap",
	0x83: "Linux", 0x85: "Linux extended", 0x8E: "Linux LVM", 0xA5: "FreeBSD", 0xA8: "macOS",
	0xAB: "macOS boot", 0xAF: "HFS+", 0xEE: "GPT protective", 0xEF: "EFI System", 0xFD: "Linux RAID",
}

func isExtended(t byte) bool { return t == 0x05 || t == 0x0F || t == 0x85 }

type mbrEntry struct {
	boot   bool
	typ    byte
	start  uint32
	length uint32
}

func parseMBREntries(sector []byte) ([4]mbrEntry, bool) {
	var out [4]mbrEntry
	if len(sector) < 512 || sector[510] != 0x55 || sector[511] != 0xAA {
		return out, false
	}
	for i := 0; i < 4; i++ {
		e := sector[446+16*i:]
		out[i] = mbrEntry{
			boot:   e[0] == 0x80,
			typ:    e[4],
			start:  binary.LittleEndian.Uint32(e[8:]),
			length: binary.LittleEndian.Uint32(e[12:]),
		}
	}
	return out, true
}

// ReadMBR returns the partitions of an MBR table, following the chain of
// extended boot records for logical partitions. ok is false when sector 0
// has no valid MBR. A protective MBR (GPT) is reported with that type.
func ReadMBR(r io.ReaderAt, size int64, sectorSize int) (parts []Partition, ok bool, err error) {
	ss := int64(sectorSize)
	buf := make([]byte, 512)
	if _, err := r.ReadAt(buf, 0); err != nil && err != io.EOF {
		return nil, false, err
	}
	entries, valid := parseMBREntries(buf)
	if !valid {
		return nil, false, nil
	}
	// A FAT or NTFS boot sector also ends in 55 AA; it isn't an MBR when
	// its "entries" are nonsense.
	for _, e := range entries {
		if e.typ != 0 && (int64(e.start)*ss >= size || e.length == 0) {
			return nil, false, nil
		}
	}
	idx := 1
	for _, e := range entries {
		if e.typ == 0 || e.length == 0 {
			continue
		}
		if isExtended(e.typ) {
			logical, err := readEBRChain(r, size, ss, int64(e.start), &idx)
			if err != nil {
				return parts, true, err
			}
			parts = append(parts, logical...)
			continue
		}
		parts = append(parts, mbrPartition(e, 0, ss, size, idx))
		idx++
	}
	return parts, true, nil
}

func mbrPartition(e mbrEntry, base, ss, size int64, idx int) Partition {
	off := (base + int64(e.start)) * ss
	n := int64(e.length) * ss
	if off+n > size {
		n = size - off
	}
	name, ok := mbrTypes[e.typ]
	if !ok {
		name = fmt.Sprintf("Type 0x%02X", e.typ)
	}
	return Partition{Index: idx, Scheme: SchemeMBR, Offset: off, Length: n, Type: name, TypeID: fmt.Sprintf("0x%02X", e.typ), Bootable: e.boot}
}

func readEBRChain(r io.ReaderAt, size, ss, extStart int64, idx *int) ([]Partition, error) {
	var out []Partition
	buf := make([]byte, 512)
	next := extStart
	for hops := 0; hops < 128; hops++ {
		if next*ss >= size {
			break
		}
		if _, err := r.ReadAt(buf, next*ss); err != nil && err != io.EOF {
			return out, err
		}
		entries, ok := parseMBREntries(buf)
		if !ok {
			break
		}
		if e := entries[0]; e.typ != 0 && e.length != 0 {
			out = append(out, mbrPartition(e, next, ss, size, *idx))
			*idx++
		}
		link := entries[1]
		if link.typ == 0 || !isExtended(link.typ) {
			break
		}
		next = extStart + int64(link.start)
	}
	return out, nil
}
