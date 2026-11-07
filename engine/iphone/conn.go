package iphone

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/xmhrnx/remanance/engine/plist"
)

func (c *conn) send(v any) error {
	body, err := plist.MarshalXML(v)
	if err != nil {
		return err
	}
	return c.sendRaw(body)
}

// sendBinary sends v as a binary property list, as the DeviceLink
// services take them.
func (c *conn) sendBinary(v any) error {
	body, err := plist.MarshalBinary(v)
	if err != nil {
		return err
	}
	return c.sendRaw(body)
}

func (c *conn) sendRaw(body []byte) error {
	h := binary.BigEndian.AppendUint32(nil, uint32(len(body)))
	_, err := c.Write(append(h, body...))
	return err
}

func (c *conn) recv() (any, error) {
	h := make([]byte, 4)
	if _, err := io.ReadFull(c, h); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(h)
	if n > 64<<20 {
		return nil, fmt.Errorf("the iPhone sent a message of %d bytes", n)
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(c, body); err != nil {
		return nil, err
	}
	return plist.Parse(body)
}
