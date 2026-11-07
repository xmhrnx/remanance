package partition

import "encoding/binary"

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
