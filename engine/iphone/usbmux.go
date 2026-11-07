package iphone

import (
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"time"

	"github.com/xmhrnx/remanance/engine/plist"
)

// mux is a connection to usbmuxd. Its messages have a 16-byte header,
// little-endian: length, version 1, type 8 for a property list, and a
// tag that the reply repeats.
type mux struct {
	conn net.Conn
	tag  uint32
}

// device is a phone usbmuxd sees.
type device struct {
	id   int64
	udid string
}

// muxError is usbmuxd's result for a request that failed.
type muxError int64

// ErrNoService means Apple's USB service isn't running, or installed.
var ErrNoService = errors.New(apple)

var apple = func() string {
	switch runtime.GOOS {
	case "windows":
		return "Apple's iPhone USB service isn't running. Install Apple Devices from the Microsoft Store (or iTunes), open it once, and connect the iPhone again"
	case "linux":
		return "usbmuxd isn't running. Install it (it's in the usbmuxd package) and connect the iPhone again"
	}
	return "the iPhone USB service isn't running"
}()

// dialMux connects to usbmuxd.
func dialMux() (net.Conn, error) {
	if a := os.Getenv("USBMUXD_SOCKET_ADDRESS"); a != "" {
		network := "tcp"
		if len(a) > 0 && a[0] == '/' {
			network = "unix"
		} else if len(a) > 5 && a[:5] == "UNIX:" {
			network, a = "unix", a[5:]
		}
		return net.DialTimeout(network, a, 3*time.Second)
	}
	var c net.Conn
	var err error
	if runtime.GOOS == "windows" {
		c, err = net.DialTimeout("tcp", "127.0.0.1:27015", 3*time.Second)
	} else {
		c, err = net.DialTimeout("unix", "/var/run/usbmuxd", 3*time.Second)
	}
	if err != nil {
		return nil, ErrNoService
	}
	return c, nil
}

func openMux() (*mux, error) {
	c, err := dialMux()
	if err != nil {
		return nil, err
	}
	return &mux{conn: c}, nil
}

func (m *mux) Close() error { return m.conn.Close() }

// request sends a message of type kind and reads the reply.
func (m *mux) request(kind string, fields map[string]any) (map[string]any, error) {
	msg := map[string]any{
		"MessageType":         kind,
		"ClientVersionString": "datadoc",
		"ProgName":            "DataDoc",
		"kLibUSBMuxVersion":   int64(3),
	}
	for k, v := range fields {
		msg[k] = v
	}
	body, err := plist.MarshalXML(msg)
	if err != nil {
		return nil, err
	}
	m.tag++
	if err := writeMux(m.conn, m.tag, body); err != nil {
		return nil, err
	}
	reply, err := readMux(m.conn)
	if err != nil {
		return nil, err
	}
	v, err := plist.Parse(reply)
	if err != nil {
		return nil, err
	}
	d := plist.Dict(v)
	if d["MessageType"] == "Result" {
		if n := plist.Int(d["Number"]); n != 0 {
			return d, muxError(n)
		}
	}
	return d, nil
}

// Error says what usbmuxd's code means.
func (e muxError) Error() string {
	switch e {
	case 2:
		return "the iPhone was disconnected"
	case 3:
		return "the iPhone refused the connection"
	}
	return fmt.Sprintf("usbmuxd error %d", int64(e))
}
