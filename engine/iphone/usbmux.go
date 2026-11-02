package iphone

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"time"

	"github.com/xmhrnx/remanance/engine/plist"
)

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

// mux is a connection to usbmuxd. Its messages have a 16-byte header,
// little-endian: length, version 1, type 8 for a property list, and a
// tag that the reply repeats.
type mux struct {
	conn net.Conn
	tag  uint32
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

func writeMux(w io.Writer, tag uint32, body []byte) error {
	h := make([]byte, 16)
	binary.LittleEndian.PutUint32(h, uint32(16+len(body)))
	binary.LittleEndian.PutUint32(h[4:], 1)
	binary.LittleEndian.PutUint32(h[8:], 8)
	binary.LittleEndian.PutUint32(h[12:], tag)
	_, err := w.Write(append(h, body...))
	return err
}

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

// muxError is usbmuxd's result for a request that failed.
type muxError int64

func (e muxError) Error() string {
	switch e {
	case 2:
		return "the iPhone was disconnected"
	case 3:
		return "the iPhone refused the connection"
	}
	return fmt.Sprintf("usbmuxd error %d", int64(e))
}

// device is a phone usbmuxd sees.
type device struct {
	id   int64
	udid string
}

// devices lists the phones connected by USB.
func devices() ([]device, error) {
	m, err := openMux()
	if err != nil {
		return nil, err
	}
	defer m.Close()
	d, err := m.request("ListDevices", nil)
	if err != nil {
		return nil, err
	}
	var out []device
	list, _ := d["DeviceList"].([]any)
	for _, x := range list {
		p := plist.Dict(plist.Dict(x)["Properties"])
		if p["ConnectionType"] != "USB" {
			continue // over Wi-Fi: too slow for a backup to be worth it
		}
		out = append(out, device{id: plist.Int(p["DeviceID"]), udid: plist.String(p["SerialNumber"])})
	}
	return out, nil
}

// connect opens a connection to a port on the phone, through usbmuxd.
func connect(id int64, port uint16) (net.Conn, error) {
	m, err := openMux()
	if err != nil {
		return nil, err
	}
	// The port goes in network byte order.
	_, err = m.request("Connect", map[string]any{"DeviceID": id, "PortNumber": int64(port>>8 | port<<8)})
	if err != nil {
		m.Close()
		return nil, err
	}
	return m.conn, nil
}

// pairRecord reads the record of this computer's pairing with the
// phone, kept by usbmuxd; nil if they haven't paired.
func pairRecord(udid string) (map[string]any, error) {
	m, err := openMux()
	if err != nil {
		return nil, err
	}
	defer m.Close()
	d, err := m.request("ReadPairRecord", map[string]any{"PairRecordID": udid})
	if err != nil {
		return nil, nil
	}
	data, _ := d["PairRecordData"].([]byte)
	v, err := plist.Parse(data)
	if err != nil {
		return nil, nil
	}
	return plist.Dict(v), nil
}

// savePairRecord gives usbmuxd a new pairing to keep.
func savePairRecord(dev device, record map[string]any) error {
	data, err := plist.MarshalXML(record)
	if err != nil {
		return err
	}
	m, err := openMux()
	if err != nil {
		return err
	}
	defer m.Close()
	_, err = m.request("SavePairRecord", map[string]any{"PairRecordID": dev.udid, "PairRecordData": data, "DeviceID": dev.id})
	return err
}

// systemBUID is the identifier usbmuxd gives this computer.
func systemBUID() (string, error) {
	m, err := openMux()
	if err != nil {
		return "", err
	}
	defer m.Close()
	d, err := m.request("ReadBUID", nil)
	if err != nil {
		return "", err
	}
	return plist.String(d["BUID"]), nil
}
