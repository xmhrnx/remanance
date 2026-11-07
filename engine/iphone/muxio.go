package iphone

import (
	"encoding/binary"
	"fmt"
	"io"
)

// writeMux writes a message to usbmuxd: its header, then its body.
func writeMux(w io.Writer, tag uint32, body []byte) error {
	h := make([]byte, 16)
	binary.LittleEndian.PutUint32(h, uint32(16+len(body)))
	binary.LittleEndian.PutUint32(h[4:], 1)
	binary.LittleEndian.PutUint32(h[8:], 8)
	binary.LittleEndian.PutUint32(h[12:], tag)
	_, err := w.Write(append(h, body...))
	return err
}

// readMux reads a message from usbmuxd: its header, then its body.
func readMux(r io.Reader) ([]byte, error) {
	h := make([]byte, 16)
	if _, err := io.ReadFull(r, h); err != nil {
		return nil, err
	}
	n := binary.LittleEndian.Uint32(h)
	if n < 16 || n > 16<<20 {
		return nil, fmt.Errorf("usbmuxd sent a message of %d bytes", n)
	}
	body := make([]byte, n-16)
	_, err := io.ReadFull(r, body)
	return body, err
}
