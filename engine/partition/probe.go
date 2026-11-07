package partition

import (
	"bytes"
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
