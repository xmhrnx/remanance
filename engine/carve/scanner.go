package carve

import (
	"bytes"
	"io"
	"sort"
	"sync"
)

// Found is a file located by its content.
type Found struct {
	Offset  int64 // on the device
	Length  int64
	Sig     *Signature
	Ext     string
	Name    string
	Partial bool
}

// Scanner checks the blocks of a linear device read for file starts.
// Files start at sector boundaries on every file system, so only those
// are checked; that keeps the scan as fast as the device can be read.
type Scanner struct {
	dev    io.ReaderAt
	size   int64
	sector int64
	sigs   []*Signature
	// byLead indexes signatures by their first two magic bytes at offset
	// zero; others (magic deeper in the file) are checked separately.
	byLead map[uint16][]*Signature
	deep   []*Signature
	skip   func(off int64) (end int64, ok bool)

	mu     sync.Mutex
	found  []Found
	resume int64 // offsets below this are inside a file already found
}

// NewScanner scans dev for the given signatures (all registered ones when
// sigs is nil). skip, if set, reports device ranges whose files are
// already known from file system metadata, so they aren't listed twice.
func NewScanner(dev io.ReaderAt, size int64, sectorSize int, sigs []*Signature, skip func(off int64) (int64, bool)) *Scanner {
	if sigs == nil {
		sigs = Signatures()
	}
	if sectorSize <= 0 {
		sectorSize = 512
	}
	s := &Scanner{dev: dev, size: size, sector: int64(sectorSize), sigs: sigs, byLead: map[uint16][]*Signature{}, skip: skip}
	for _, sig := range sigs {
		lead := false
		for _, m := range sig.Magics {
			if m.Offset == 0 && len(m.Bytes) >= 2 {
				k := uint16(m.Bytes[0])<<8 | uint16(m.Bytes[1])
				s.byLead[k] = append(s.byLead[k], sig)
				lead = true
			}
		}
		if !lead {
			s.deep = append(s.deep, sig)
		}
	}
	return s
}

func (s *Scanner) matches(sig *Signature, b []byte) bool {
	for _, m := range sig.Magics {
		if m.Offset+len(m.Bytes) <= len(b) && bytes.Equal(b[m.Offset:m.Offset+len(m.Bytes)], m.Bytes) {
			return true
		}
	}
	return false
}

// Block examines a block read at device offset pos. Blocks must be passed
// in increasing order of pos.
func (s *Scanner) Block(buf []byte, pos int64) {
	for i := int64(0); i+16 <= int64(len(buf)); i += s.sector {
		off := pos + i
		s.mu.Lock()
		inside := off < s.resume
		s.mu.Unlock()
		if inside {
			continue
		}
		b := buf[i:]
		cands := s.byLead[uint16(b[0])<<8|uint16(b[1])]
		for _, sig := range cands {
			if s.try(sig, b, off) {
				goto next
			}
		}
		for _, sig := range s.deep {
			if s.try(sig, b, off) {
				break
			}
		}
	next:
	}
}

func (s *Scanner) try(sig *Signature, b []byte, off int64) bool {
	if !s.matches(sig, b) {
		return false
	}
	if s.skip != nil {
		if end, known := s.skip(off); known {
			s.mu.Lock()
			s.resume = max(s.resume, end)
			s.mu.Unlock()
			return true
		}
	}
	limit := min(sig.MaxSize, s.size-off)
	m := sig.Parse(io.NewSectionReader(s.dev, off, limit), limit)
	if m.Length <= 0 {
		return false
	}
	f := Found{Offset: off, Length: m.Length, Sig: sig, Ext: m.Ext, Name: m.Name, Partial: m.Partial}
	if f.Ext == "" {
		f.Ext = sig.Ext
	}
	s.mu.Lock()
	s.found = append(s.found, f)
	if !m.Partial {
		// Don't look for files inside this one: what's inside a video or
		// an archive isn't a separate lost file.
		s.resume = max(s.resume, off+m.Length)
	}
	s.mu.Unlock()
	return true
}

// Found returns the files found so far, by offset.
func (s *Scanner) Found() []Found {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]Found(nil), s.found...)
	sort.Slice(out, func(i, j int) bool { return out[i].Offset < out[j].Offset })
	return out
}

// Count is how many files were found so far.
func (s *Scanner) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.found)
}
