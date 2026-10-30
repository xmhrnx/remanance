// Package plist reads Apple property lists, binary and XML, as iPhone
// backups keep their descriptions in. Values come back as
// map[string]any, []any, string, int64, float64, bool, time.Time,
// []byte and UID.
package plist

import (
	"bytes"
	"errors"
)

// UID is a reference to another object in an NSKeyedArchiver archive.
type UID uint64

// ErrFormat means the data isn't a property list this package reads.
var ErrFormat = errors.New("plist: not a property list")

// Parse reads a binary or XML property list.
func Parse(data []byte) (any, error) {
	if bytes.HasPrefix(data, []byte("bplist00")) {
		return parseBinary(data)
	}
	return parseXML(data)
}

// Dict returns v as a dictionary, or nil.
func Dict(v any) map[string]any {
	d, _ := v.(map[string]any)
	return d
}

// Int returns v as an integer, or 0.
func Int(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case float64:
		return int64(n)
	case UID:
		return int64(n)
	}
	return 0
}

// String returns v as a string, or "".
func String(v any) string {
	s, _ := v.(string)
	return s
}
