package fat

import (
	"encoding/binary"
	"strings"
	"time"
	"unicode/utf16"
)

const (
	attrReadOnly  = 0x01
	attrHidden    = 0x02
	attrSystem    = 0x04
	attrVolumeID  = 0x08
	attrDirectory = 0x10
	attrLFN       = 0x0F

	entryDeleted = 0xE5
)

// dirent is a short directory entry together with the long name that
// preceded it.
type dirent struct {
	name     string
	deleted  bool
	isDir    bool
	cluster  uint32
	size     int64
	created  time.Time
	modified time.Time
	accessed time.Time
	offset   int64 // where the entry is in the volume, unique per file
}

func dosTime(date, tm uint16) time.Time {
	if date == 0 {
		return time.Time{}
	}
	return time.Date(1980+int(date>>9), time.Month(date>>5&15), int(date&31),
		int(tm>>11), int(tm>>5&63), int(tm&31)*2, 0, time.Local)
}

func lfnChecksum(short []byte) byte {
	var sum byte
	for _, c := range short[:11] {
		sum = (sum&1)<<7 + sum>>1 + c
	}
	return sum
}

// shortName renders an 8.3 name, applying the lower-case flags Windows
// keeps in byte 12.
func shortName(e []byte) string {
	base := strings.TrimRight(string(e[0:8]), " ")
	ext := strings.TrimRight(string(e[8:11]), " ")
	if e[12]&0x08 != 0 {
		base = strings.ToLower(base)
	}
	if e[12]&0x10 != 0 {
		ext = strings.ToLower(ext)
	}
	if ext == "" {
		return base
	}
	return base + "." + ext
}

// shortNameFirst returns the bytes Windows could have put first in the
// 8.3 name it made for a long name.
func shortNameFirst(long string) []byte {
	s := strings.TrimLeft(strings.ReplaceAll(long, " ", ""), ".")
	if s == "" {
		return []byte{'_'}
	}
	c := s[0]
	if c >= 'a' && c <= 'z' {
		c -= 'a' - 'A'
	}
	if c < 0x80 && strings.IndexByte("+,;=[]", c) < 0 {
		return []byte{c, '_'}
	}
	return []byte{'_'}
}

type lfnPart struct {
	seq      int
	checksum byte
	chars    []uint16
}

func parseLFN(e []byte) lfnPart {
	p := lfnPart{seq: int(e[0] & 0x1F), checksum: e[13]}
	for _, r := range [][2]int{{1, 11}, {14, 26}, {28, 32}} {
		for i := r[0]; i < r[1]; i += 2 {
			p.chars = append(p.chars, binary.LittleEndian.Uint16(e[i:]))
		}
	}
	return p
}

func assembleLFN(parts []lfnPart) string {
	// Parts precede the short entry in reverse order: the last part
	// first.
	var u []uint16
	for i := len(parts) - 1; i >= 0; i-- {
		u = append(u, parts[i].chars...)
	}
	for i, c := range u {
		if c == 0 || c == 0xFFFF {
			u = u[:i]
			break
		}
	}
	return string(utf16.Decode(u))
}

// plausible rejects 32 bytes that aren't a short directory entry: names
// with control characters or bytes FAT forbids, impossible dates.
func plausible(e []byte, deleted bool) bool {
	start := 0
	if deleted {
		start = 1
	}
	for i := start; i < 11; i++ {
		c := e[i]
		if c < 0x20 || strings.IndexByte("\"*+,/:;<=>?\\[]|", c) >= 0 {
			return false
		}
	}
	if e[11]&0xC0 != 0 {
		return false
	}
	if d := binary.LittleEndian.Uint16(e[24:]); d != 0 && (d&31 == 0 || d>>5&15 == 0 || d>>5&15 > 12) {
		return false
	}
	return true
}

// parseDir returns the entries of a directory's raw bytes, deleted ones
// included. base is the volume offset of raw[0]. Entries a deleted name's
// LFN can't vouch for keep their 8.3 name with "_" for the lost first
// letter.
func parseDir(raw []byte, base int64, includeDeleted bool) (out []dirent, valid int) {
	var lfn []lfnPart
	for off := 0; off+32 <= len(raw); off += 32 {
		e := raw[off : off+32]
		first := e[0]
		if first == 0 {
			// End of the directory. Anything after is left from before
			// and isn't trustworthy enough to list.
			break
		}
		deleted := first == entryDeleted
		if e[11] == attrLFN {
			if deleted && !includeDeleted {
				lfn = nil
				continue
			}
			p := parseLFN(e)
			if !deleted && first&0x40 != 0 {
				lfn = lfn[:0]
			}
			lfn = append(lfn, p)
			continue
		}
		parts := lfn
		lfn = nil
		if first == '.' || e[11]&attrVolumeID != 0 {
			continue
		}
		if deleted && !includeDeleted {
			continue
		}
		if !plausible(e, deleted) {
			continue
		}
		valid++
		d := dirent{
			deleted:  deleted,
			isDir:    e[11]&attrDirectory != 0,
			cluster:  uint32(binary.LittleEndian.Uint16(e[20:]))<<16 | uint32(binary.LittleEndian.Uint16(e[26:])),
			size:     int64(binary.LittleEndian.Uint32(e[28:])),
			created:  dosTime(binary.LittleEndian.Uint16(e[16:]), binary.LittleEndian.Uint16(e[14:])),
			modified: dosTime(binary.LittleEndian.Uint16(e[24:]), binary.LittleEndian.Uint16(e[22:])),
			accessed: dosTime(binary.LittleEndian.Uint16(e[18:]), 0),
			offset:   base + int64(off),
		}
		short := append([]byte(nil), e[:11]...)
		if first == 0x05 {
			short[0] = entryDeleted // 0x05 stands for a real 0xE5 first byte
		}
		long := ""
		if len(parts) > 0 {
			if deleted {
				// The short name's first letter is gone. Windows derives
				// it from the long name, so that letter (or "_" for a
				// non-ASCII one) must make the checksum match. Trying
				// every byte would match stale long names by chance.
				candidate := assembleLFN(parts)
				for _, c := range shortNameFirst(candidate) {
					short[0] = c
					if lfnChecksum(short) == parts[0].checksum {
						long = candidate
						break
					}
				}
				if long == "" {
					short[0] = '_'
				}
			} else if lfnChecksum(short) == parts[0].checksum {
				long = assembleLFN(parts)
			}
		} else if deleted {
			short[0] = '_'
		}
		if long != "" {
			d.name = long
		} else {
			tmp := append(append([]byte(nil), short...), e[11:]...)
			d.name = shortName(tmp)
		}
		if d.name == "" {
			continue
		}
		out = append(out, d)
	}
	return out, valid
}
