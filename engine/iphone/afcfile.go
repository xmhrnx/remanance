package iphone

import (
	"encoding/binary"
	"io"
)

// ReadAt seeks and reads. The two requests go together, so another
// read can't come between them.
func (f *afcFile) ReadAt(p []byte, off int64) (int, error) {
	n := 0
	for n < len(p) {
		got, err := f.read(off+int64(n), min(len(p)-n, afcMaxRead))
		n += copy(p[n:], got)
		if err != nil {
			return n, err
		}
		if len(got) == 0 {
			return n, io.EOF
		}
	}
	return n, nil
}

func (f *afcFile) read(off int64, want int) ([]byte, error) {
	f.a.mu.Lock()
	defer f.a.mu.Unlock()
	seek := binary.LittleEndian.AppendUint64(nil, f.handle)
	seek = binary.LittleEndian.AppendUint64(seek, 0) // from the start
	seek = binary.LittleEndian.AppendUint64(seek, uint64(off))
	if _, _, _, err := f.a.roundTrip(afcSeek, seek, nil); err != nil {
		return nil, err
	}
	args := binary.LittleEndian.AppendUint64(nil, f.handle)
	args = binary.LittleEndian.AppendUint64(args, uint64(want))
	_, _, data, err := f.a.roundTrip(afcRead, args, nil)
	return data, err
}

func (f *afcFile) Close() error {
	_, _, _, err := f.a.request(afcClose, binary.LittleEndian.AppendUint64(nil, f.handle), nil)
	return err
}
