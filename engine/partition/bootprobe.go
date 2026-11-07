package partition

import "encoding/binary"

// probeBoot recognizes a boot sector (NTFS, exFAT, FAT) in a 512-byte
// sector found at pos.
func probeBoot(s []byte, pos int64) (Volume, bool) {
	if len(s) < 512 {
		return Volume{}, false
	}
	switch {
	case string(s[3:11]) == "NTFS    ":
		bps := int(binary.LittleEndian.Uint16(s[11:]))
		if !isPow2(bps) || bps < 256 || bps > 4096 || s[510] != 0x55 || s[511] != 0xAA {
			return Volume{}, false
		}
		total := int64(binary.LittleEndian.Uint64(s[0x28:]))
		if total <= 0 {
			return Volume{}, false
		}
		// The volume is one sector larger than it says: the backup boot
		// sector sits just past the reported end.
		return Volume{FS: "NTFS", Offset: pos, Length: (total + 1) * int64(bps)}, true
	case string(s[3:11]) == "EXFAT   ":
		shift := s[108]
		if shift < 9 || shift > 12 {
			return Volume{}, false
		}
		n := int64(binary.LittleEndian.Uint64(s[72:]))
		return Volume{FS: "exFAT", Offset: pos, Length: n << shift}, n > 0
	}
	return probeFAT(s, pos)
}

func probeFAT(s []byte, pos int64) (Volume, bool) {
	if s[510] != 0x55 || s[511] != 0xAA || (s[0] != 0xEB && s[0] != 0xE9) {
		return Volume{}, false
	}
	bps := int(binary.LittleEndian.Uint16(s[11:]))
	spc := int(s[13])
	reserved := binary.LittleEndian.Uint16(s[14:])
	fats := s[16]
	media := s[21]
	if !isPow2(bps) || bps < 512 || bps > 4096 || !isPow2(spc) || reserved == 0 || fats == 0 || fats > 2 || media < 0xF0 {
		return Volume{}, false
	}
	total := int64(binary.LittleEndian.Uint16(s[19:]))
	if total == 0 {
		total = int64(binary.LittleEndian.Uint32(s[32:]))
	}
	if total == 0 {
		return Volume{}, false
	}
	v := Volume{Offset: pos, Length: total * int64(bps)}
	if binary.LittleEndian.Uint16(s[22:]) == 0 { // sectors per FAT 0: FAT32
		v.FS = "FAT32"
		if s[66] == 0x29 {
			v.Label = trimLabel(s[71:82])
		}
	} else {
		fatSectors := int64(binary.LittleEndian.Uint16(s[22:]))
		rootSectors := (int64(binary.LittleEndian.Uint16(s[17:]))*32 + int64(bps) - 1) / int64(bps)
		data := total - int64(reserved) - int64(fats)*fatSectors - rootSectors
		if data <= 0 {
			return Volume{}, false
		}
		if data/int64(spc) < 4085 {
			v.FS = "FAT12"
		} else {
			v.FS = "FAT16"
		}
		if s[38] == 0x29 {
			v.Label = trimLabel(s[43:54])
		}
	}
	return v, true
}
