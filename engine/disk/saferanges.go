package disk

import "sort"

// markBad records a span, merged with the spans it touches: the same
// sectors are often read more than once (the search, then the file
// systems on them), and must count once.
func (s *SafeReader) markBad(off, n int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	end := off + n
	i := sort.Search(len(s.bad), func(i int) bool { return s.bad[i].Offset+s.bad[i].Length >= off })
	j := i
	for j < len(s.bad) && s.bad[j].Offset <= end {
		off = min(off, s.bad[j].Offset)
		end = max(end, s.bad[j].Offset+s.bad[j].Length)
		j++
	}
	s.bad = append(s.bad[:i], append([]Range{{off, end - off}}, s.bad[j:]...)...)
}

// BadRanges returns the unreadable spans found so far, sorted.
func (s *SafeReader) BadRanges() []Range {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Range(nil), s.bad...)
}

// BadBytes is the total size of the unreadable spans.
func (s *SafeReader) BadBytes() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	var t int64
	for _, r := range s.bad {
		t += r.Length
	}
	return t
}
