package rename

import (
	"encoding/binary"
	"io"
	"os"
	"strings"
	"time"
)

// photoDate reads when a JPEG photo was taken from its EXIF data:
// DateTimeOriginal, or else the date the camera saved it.
func photoDate(p string) (time.Time, bool) {
	ext := strings.ToLower(p[strings.LastIndexByte(p, '.')+1:])
	if ext != "jpg" && ext != "jpeg" {
		return time.Time{}, false
	}
	f, err := os.Open(p)
	if err != nil {
		return time.Time{}, false
	}
	defer f.Close()
	// The EXIF block comes early, in an APP1 segment.
	head := make([]byte, 128<<10)
	n, _ := io.ReadFull(f, head)
	head = head[:n]
	if len(head) < 4 || head[0] != 0xFF || head[1] != 0xD8 {
		return time.Time{}, false
	}
	for i := 2; i+4 <= len(head); {
		if head[i] != 0xFF {
			return time.Time{}, false
		}
		marker := head[i+1]
		size := int(binary.BigEndian.Uint16(head[i+2:]))
		if marker == 0xDA || size < 2 || i+2+size > len(head) {
			return time.Time{}, false
		}
		seg := head[i+4 : i+2+size]
		if marker == 0xE1 && len(seg) > 6 && string(seg[:6]) == "Exif\x00\x00" {
			return tiffDate(seg[6:])
		}
		i += 2 + size
	}
	return time.Time{}, false
}

// tiffDate finds the dates in a TIFF block's first directory and its
// EXIF directory.
func tiffDate(t []byte) (time.Time, bool) {
	if len(t) < 8 {
		return time.Time{}, false
	}
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return time.Time{}, false
	}
	str := func(count, off uint32) string {
		if count < 19 || int(off)+19 > len(t) {
			return ""
		}
		return string(t[off : off+19])
	}
	var saved, exif string
	var exifDir uint32
	dir := func(off uint32, wantTaken bool) {
		if int(off)+2 > len(t) {
			return
		}
		n := int(bo.Uint16(t[off:]))
		for k := range n {
			e := int(off) + 2 + k*12
			if e+12 > len(t) {
				return
			}
			tag, count, val := bo.Uint16(t[e:]), bo.Uint32(t[e+4:]), bo.Uint32(t[e+8:])
			switch {
			case tag == 0x0132 && !wantTaken:
				saved = str(count, val)
			case tag == 0x8769 && !wantTaken:
				exifDir = val
			case tag == 0x9003 && wantTaken:
				exif = str(count, val)
			}
		}
	}
	dir(bo.Uint32(t[4:]), false)
	if exifDir != 0 {
		dir(exifDir, true)
	}
	for _, s := range []string{exif, saved} {
		if when, err := time.ParseInLocation("2006:01:02 15:04:05", s, time.Local); err == nil {
			return when, true
		}
	}
	return time.Time{}, false
}
