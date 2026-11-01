package devsync

import (
	"bufio"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
)

// Messages go as frames: their length, four bytes big-endian, counting
// the type byte that follows, then the body, JSON but for a response.
const (
	msgHello    = 1
	msgIndex    = 2
	msgRequest  = 3
	msgResponse = 4
)

// maxFrame is the largest frame accepted: an index of many files.
const maxFrame = 64 << 20

// blockSize is how much of a file one request asks for.
const blockSize = 512 << 10

// hello says who a device is and which folders it shares with the one
// it's talking to.
type hello struct {
	Name    string        `json:"name"`
	Folders []folderOffer `json:"folders"`
}

type folderOffer struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// indexMsg carries a folder's files: all of them when Full, or those
// that changed.
type indexMsg struct {
	Folder string `json:"folder"`
	Full   bool   `json:"full"`
	Files  []File `json:"files"`
}

// request asks for a block of a file, at the version with Hash.
type request struct {
	ID     uint64 `json:"id"`
	Folder string `json:"folder"`
	Name   string `json:"name"`
	Hash   string `json:"hash"`
	Offset int64  `json:"offset"`
	Size   int    `json:"size"`
}

// conn is a connection to another device.
type conn struct {
	tls     *tls.Conn
	r       *bufio.Reader
	wmu     sync.Mutex
	device  string // its ID
	name    string
	addr    string
	inbound bool
	// folders are the IDs it shares with this device, from its hello.
	folders map[string]bool

	mu      sync.Mutex
	next    uint64
	waiting map[uint64]chan response
	closed  bool
}

type response struct {
	data []byte
	err  error
}

func newConn(t *tls.Conn, device string, inbound bool) *conn {
	return &conn{tls: t, r: bufio.NewReaderSize(t, 64<<10), device: device, addr: t.RemoteAddr().String(),
		inbound: inbound, folders: map[string]bool{}, waiting: map[uint64]chan response{}}
}

func (c *conn) send(typ byte, body []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	h := binary.BigEndian.AppendUint32(nil, uint32(len(body)+1))
	h = append(h, typ)
	if _, err := c.tls.Write(append(h, body...)); err != nil {
		return err
	}
	return nil
}

func (c *conn) sendJSON(typ byte, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.send(typ, b)
}

func (c *conn) read() (byte, []byte, error) {
	h := make([]byte, 5)
	if _, err := io.ReadFull(c.r, h); err != nil {
		return 0, nil, err
	}
	n := binary.BigEndian.Uint32(h)
	if n < 1 || n > maxFrame {
		return 0, nil, fmt.Errorf("a frame of %d bytes", n)
	}
	body := make([]byte, n-1)
	if _, err := io.ReadFull(c.r, body); err != nil {
		return 0, nil, err
	}
	return h[4], body, nil
}

// fetch asks for a block and waits for it.
func (c *conn) fetch(folder, name, hash string, off int64, size int) ([]byte, error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, errClosed
	}
	c.next++
	id := c.next
	ch := make(chan response, 1)
	c.waiting[id] = ch
	c.mu.Unlock()
	if err := c.sendJSON(msgRequest, request{ID: id, Folder: folder, Name: name, Hash: hash, Offset: off, Size: size}); err != nil {
		return nil, err
	}
	r := <-ch
	return r.data, r.err
}

// respond answers request id with data, or with an error.
func (c *conn) respond(id uint64, data []byte, err error) error {
	body := binary.BigEndian.AppendUint64(nil, id)
	if err != nil {
		body = append(body, 0)
		body = append(body, err.Error()...)
	} else {
		body = append(body, 1)
		body = append(body, data...)
	}
	return c.send(msgResponse, body)
}

// delivered hands a response to the fetch waiting for it.
func (c *conn) delivered(body []byte) {
	if len(body) < 9 {
		return
	}
	id := binary.BigEndian.Uint64(body)
	r := response{data: body[9:]}
	if body[8] == 0 {
		r = response{err: errors.New(string(body[9:]))}
	}
	c.mu.Lock()
	ch := c.waiting[id]
	delete(c.waiting, id)
	c.mu.Unlock()
	if ch != nil {
		ch <- r
	}
}

func (c *conn) close() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	waiting := c.waiting
	c.waiting = map[uint64]chan response{}
	c.mu.Unlock()
	c.tls.Close()
	for _, ch := range waiting {
		ch <- response{err: errClosed}
	}
}

var errClosed = errors.New("the other device disconnected")
