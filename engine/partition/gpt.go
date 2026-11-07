package partition

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"strings"
	"unicode/utf16"
)

type gptHeader struct {
	entriesLBA int64
	count      int
	entrySize  int
	entriesCRC uint32
}

var gptTypes = map[string]string{
	"C12A7328-F81F-11D2-BA4B-00A0C93EC93B": "EFI System",
	"E3C9E316-0B5C-4DB8-817D-F92DF00215AE": "Microsoft reserved",
	"EBD0A0A2-B9E5-4433-87C0-68B6B72699C7": "Basic data",
	"DE94BBA4-06D1-4D40-A16A-BFD50179D6AC": "Windows RE",
	"5808C8AA-7E8F-42E0-85D2-E1E90434CFB3": "LDM metadata",
	"AF9B60A0-1431-4F62-BC68-3311714A69AD": "LDM data",
	"0FC63DAF-8483-4772-8E79-3D69D8477DE4": "Linux file system",
	"0657FD6D-A4AB-43C4-84E5-0933C84B4F4F": "Linux swap",
	"E6D6D379-F507-44C2-A23C-238F2A3DF928": "Linux LVM",
	"A19D880F-05FC-4D3B-A006-743F0F84911E": "Linux RAID",
	"4F68BCE3-E8CD-4DB1-96E7-FBCAF984B709": "Linux root (x86-64)",
	"933AC7E1-2EB4-4F13-B844-0E14E2AEF915": "Linux home",
	"7C3457EF-0000-11AA-AA11-00306543ECAC": "APFS",
	"48465300-0000-11AA-AA11-00306543ECAC": "HFS+",
	"426F6F74-0000-11AA-AA11-00306543ECAC": "Apple boot",
	"52637672-7900-11AA-AA11-00306543ECAC": "Apple recovery",
	"516E7CB4-6ECF-11D6-8FF8-00022D09712B": "FreeBSD data",
	"21686148-6449-6E6F-744E-656564454649": "BIOS boot",
	"E75CAF8F-F680-4CEE-AFA3-B001E56EFC2D": "Storage Spaces",
	"DC76D94B-01FB-4D9F-A7B2-6A3BA2D6EE5B": "Android data", // seen on some phones' SD cards
}

func guidString(b []byte) string {
	return fmt.Sprintf("%08X-%04X-%04X-%02X%02X-%02X%02X%02X%02X%02X%02X",
		binary.LittleEndian.Uint32(b[0:]), binary.LittleEndian.Uint16(b[4:]), binary.LittleEndian.Uint16(b[6:]),
		b[8], b[9], b[10], b[11], b[12], b[13], b[14], b[15])
}

func readGPTHeader(r io.ReaderAt, lba, ss int64) (gptHeader, bool) {
	buf := make([]byte, ss)
	if _, err := r.ReadAt(buf, lba*ss); err != nil && err != io.EOF {
		return gptHeader{}, false
	}
	if string(buf[:8]) != "EFI PART" {
		return gptHeader{}, false
	}
	hsize := binary.LittleEndian.Uint32(buf[12:])
	if hsize < 92 || int64(hsize) > ss {
		return gptHeader{}, false
	}
	want := binary.LittleEndian.Uint32(buf[16:])
	h := make([]byte, hsize)
	copy(h, buf[:hsize])
	clear(h[16:20])
	if crc32.ChecksumIEEE(h) != want {
		return gptHeader{}, false
	}
	return gptHeader{
		entriesLBA: int64(binary.LittleEndian.Uint64(buf[72:])),
		count:      int(binary.LittleEndian.Uint32(buf[80:])),
		entrySize:  int(binary.LittleEndian.Uint32(buf[84:])),
		entriesCRC: binary.LittleEndian.Uint32(buf[88:]),
	}, true
}

// ReadGPT returns the partitions of a GUID partition table. When the
// primary header is damaged it falls back to the backup at the end of the
// device; damaged reports that the backup had to be used.
func ReadGPT(r io.ReaderAt, size int64, sectorSize int) (parts []Partition, ok, damaged bool, err error) {
	ss := int64(sectorSize)
	h, ok := readGPTHeader(r, 1, ss)
	if !ok {
		h, ok = readGPTHeader(r, size/ss-1, ss)
		damaged = ok
	}
	if !ok {
		return nil, false, false, nil
	}
	if h.entrySize < 128 || h.count <= 0 || h.count > 1024 {
		return nil, false, damaged, nil
	}
	table := make([]byte, h.count*h.entrySize)
	if _, err := r.ReadAt(table, h.entriesLBA*ss); err != nil && err != io.EOF {
		return nil, true, damaged, err
	}
	if crc32.ChecksumIEEE(table) != h.entriesCRC {
		damaged = true
	}
	for i := 0; i < h.count; i++ {
		e := table[i*h.entrySize:]
		typ := guidString(e[0:16])
		if typ == "00000000-0000-0000-0000-000000000000" {
			continue
		}
		first := int64(binary.LittleEndian.Uint64(e[32:]))
		last := int64(binary.LittleEndian.Uint64(e[40:]))
		if last < first || first*ss >= size {
			continue
		}
		u := make([]uint16, 36)
		for j := range u {
			u[j] = binary.LittleEndian.Uint16(e[56+2*j:])
		}
		name := strings.TrimRight(string(utf16.Decode(u)), "\x00")
		tname, known := gptTypes[typ]
		if !known {
			tname = "GPT partition"
		}
		n := (last - first + 1) * ss
		if first*ss+n > size {
			n = size - first*ss
		}
		parts = append(parts, Partition{
			Index: len(parts) + 1, Scheme: SchemeGPT, Offset: first * ss, Length: n,
			Type: tname, TypeID: typ, Name: name,
		})
	}
	return parts, true, damaged, nil
}
