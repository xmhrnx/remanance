package partition

import (
	"bytes"
	"encoding/binary"
	"io"
	"strings"
)

// Volume is a file system recognized from its boot sector or superblock.
type Volume struct {
	FS     string // "NTFS", "FAT32", "exFAT", "ext4", "APFS", "HFS+", ...
	Label  string
	Offset int64 // where the volume starts on the device
	Length int64 // its size by its own metadata
	// Backup is set when the volume was recognized from a backup copy of
	// its boot sector or superblock (the primary may be gone).
	Backup bool
}

func isPow2(v int) bool { return v > 0 && v&(v-1) == 0 }

func trimLabel(b []byte) string {
	if i := bytes.IndexByte(b, 0); i >= 0 {
		b = b[:i]
	}
	s := strings.TrimSpace(string(b))
	if s == "NO NAME" {
		return ""
	}
	return s
}

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

// probeExt recognizes an ext2/3/4 superblock found at sbPos, which may be
// a backup copy in a later block group.
func probeExt(sb []byte, sbPos int64) (Volume, bool) {
	if len(sb) < 1024 || binary.LittleEndian.Uint16(sb[56:]) != 0xEF53 {
		return Volume{}, false
	}
	logBS := binary.LittleEndian.Uint32(sb[24:])
	if logBS > 6 {
		return Volume{}, false
	}
	bs := int64(1024) << logBS
	bpg := int64(binary.LittleEndian.Uint32(sb[32:]))
	ipg := binary.LittleEndian.Uint32(sb[40:])
	if bpg == 0 || ipg == 0 || bpg > 8*bs {
		return Volume{}, false
	}
	blocks := int64(binary.LittleEndian.Uint32(sb[4:]))
	incompat := binary.LittleEndian.Uint32(sb[96:])
	if incompat&0x80 != 0 { // 64bit
		blocks |= int64(binary.LittleEndian.Uint32(sb[0x150:])) << 32
	}
	firstData := int64(binary.LittleEndian.Uint32(sb[20:]))
	group := int64(binary.LittleEndian.Uint16(sb[0x5A:]))
	var start int64
	if group == 0 {
		start = sbPos - 1024
	} else {
		start = sbPos - (group*bpg+firstData)*bs
	}
	if start < 0 {
		return Volume{}, false
	}
	compat := binary.LittleEndian.Uint32(sb[92:])
	fs := "ext2"
	switch {
	case incompat&0x40 != 0 || incompat&0x200 != 0: // extents or flex_bg
		fs = "ext4"
	case compat&0x4 != 0: // has_journal
		fs = "ext3"
	}
	return Volume{FS: fs, Label: trimLabel(sb[120:136]), Offset: start, Length: blocks * bs, Backup: group != 0}, true
}

func probeAPFS(b []byte, pos int64) (Volume, bool) {
	if len(b) < 48 || string(b[32:36]) != "NXSB" {
		return Volume{}, false
	}
	bs := int64(binary.LittleEndian.Uint32(b[36:]))
	n := int64(binary.LittleEndian.Uint64(b[40:]))
	if !isPow2(int(bs)) || bs < 4096 || n == 0 {
		return Volume{}, false
	}
	return Volume{FS: "APFS", Offset: pos, Length: bs * n}, true
}

func probeHFS(h []byte, pos int64) (Volume, bool) {
	if len(h) < 512 || !(h[0] == 'H' && (h[1] == '+' || h[1] == 'X')) {
		return Volume{}, false
	}
	bs := int64(binary.BigEndian.Uint32(h[40:]))
	n := int64(binary.BigEndian.Uint32(h[44:]))
	if !isPow2(int(bs)) || bs < 512 || n == 0 {
		return Volume{}, false
	}
	return Volume{FS: "HFS+", Offset: pos - 1024, Length: bs * n}, true
}

// Probe identifies the file system that starts at off.
func Probe(r io.ReaderAt, off int64) (Volume, bool) {
	buf := make([]byte, 4096)
	n, _ := r.ReadAt(buf, off)
	if n < 2048 {
		return Volume{}, false
	}
	if v, ok := probeBoot(buf[:512], off); ok {
		return v, true
	}
	if v, ok := probeAPFS(buf, off); ok {
		return v, true
	}
	if v, ok := probeExt(buf[1024:2048], off+1024); ok && v.Offset == off {
		return v, true
	}
	if v, ok := probeHFS(buf[1024:2048], off+1024); ok {
		return v, true
	}
	return Volume{}, false
}

// ProbeSector checks one sector-aligned block met during a linear scan.
// buf starts at device offset pos and holds at least 1024 bytes. A match
// may be a backup copy; Finder works out where its volume starts.
func ProbeSector(buf []byte, pos int64) (Volume, bool) {
	if len(buf) < 1024 {
		return Volume{}, false
	}
	if v, ok := probeBoot(buf[:512], pos); ok {
		return v, true
	}
	if v, ok := probeExt(buf[:1024], pos); ok {
		return v, true
	}
	if v, ok := probeAPFS(buf, pos); ok {
		return v, true
	}
	return Volume{}, false
}
