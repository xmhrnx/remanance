// Package carve finds files by their content, for space no file system
// describes any more: formatted drives, overwritten tables, free space.
//
// Unlike carvers that cut from a header to the next header or a fixed
// size, every format here is parsed: its structure says where the file
// ends, and a structure that doesn't hold up means no file.
package carve

import "io"

// Category groups file types the way people look for them.
type Category string

const (
	Pictures  Category = "Pictures"
	Videos    Category = "Videos"
	Audio     Category = "Audio"
	Documents Category = "Documents"
	Archives  Category = "Archives"
	Databases Category = "Databases"
	Other     Category = "Other"
)

// Magic is a byte pattern at a fixed offset from the start of a file.
type Magic struct {
	Offset int
	Bytes  []byte
}

// Match is what a format's parser says about a candidate.
type Match struct {
	Length int64  // bytes from the start, 0 when not a valid file
	Ext    string // overrides the signature's extension (docx for a zip)
	Name   string // a name taken from the file's own metadata, if any
	// Partial is set when the file's start is valid but its end couldn't
	// be confirmed; Length is then a best guess.
	Partial bool
}

// Signature describes one file format.
type Signature struct {
	Name     string
	Ext      string
	Category Category
	Magics   []Magic
	// MaxSize bounds the parse and partial guesses.
	MaxSize int64
	// Parse reads the candidate at offset 0 of r, at most limit bytes,
	// and reports its length.
	Parse func(r io.ReaderAt, limit int64) Match
}

var registry []*Signature

// Register adds a signature to the set the scanner looks for.
func Register(s *Signature) { registry = append(registry, s) }

// Signatures returns every registered signature.
func Signatures() []*Signature { return append([]*Signature(nil), registry...) }
