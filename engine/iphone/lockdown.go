package iphone

import (
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/xmhrnx/remanance/engine/plist"
)

// lockdownPort is where the phone's lockdown service listens: it says
// what the phone is, pairs it with computers and starts its other
// services.
const lockdownPort = 62078

// ErrLocked means the phone has to be unlocked first.
var ErrLocked = errors.New("unlock the iPhone, then try again")

// ErrDenied means the phone's owner tapped Don't Trust.
var ErrDenied = errors.New("the iPhone doesn't trust this computer. Disconnect it, connect it again and tap Trust")

// conn sends property lists to a service on the phone, each after its
// length, four bytes big-endian, and reads its replies the same way.
type conn struct {
	net.Conn
}

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

// lockdown is a session with the phone's lockdown service.
type lockdown struct {
	*conn
	dev    device
	record map[string]any // the pairing, once paired
}

func openLockdown(dev device) (*lockdown, error) {
	c, err := connect(dev.id, lockdownPort)
	if err != nil {
		return nil, err
	}
	return &lockdown{conn: &conn{c}, dev: dev}, nil
}

// request sends a request and reads its reply, which says Error if it
// failed.
func (l *lockdown) request(name string, fields map[string]any) (map[string]any, error) {
	msg := map[string]any{"Label": "Remanence", "Request": name}
	for k, v := range fields {
		msg[k] = v
	}
	if err := l.send(msg); err != nil {
		return nil, err
	}
	v, err := l.recv()
	if err != nil {
		return nil, err
	}
	d := plist.Dict(v)
	if e := plist.String(d["Error"]); e != "" {
		return d, lockdownError(e)
	}
	return d, nil
}

type lockdownError string

func (e lockdownError) Error() string {
	switch e {
	case "PasswordProtected":
		return ErrLocked.Error()
	case "UserDeniedPairing":
		return ErrDenied.Error()
	case "InvalidHostID":
		return "the iPhone no longer trusts this computer: disconnect it, connect it again and tap Trust"
	}
	return "the iPhone said " + string(e)
}

func (e lockdownError) Is(target error) bool {
	return e == "PasswordProtected" && target == ErrLocked || e == "UserDeniedPairing" && target == ErrDenied
}

// value reads one of the phone's values, from a domain or, with domain
// "", the main one.
func (l *lockdown) value(domain, key string) any {
	f := map[string]any{"Key": key}
	if domain != "" {
		f["Domain"] = domain
	}
	d, err := l.request("GetValue", f)
	if err != nil {
		return nil
	}
	return d["Value"]
}

// startSession starts a session with the pairing, after which the
// connection is encrypted.
func (l *lockdown) startSession() error {
	d, err := l.request("StartSession", map[string]any{
		"HostID":     plist.String(l.record["HostID"]),
		"SystemBUID": plist.String(l.record["SystemBUID"]),
	})
	if err != nil {
		return err
	}
	if b, _ := d["EnableSessionSSL"].(bool); b {
		t, err := l.tls(l.Conn)
		if err != nil {
			return err
		}
		l.Conn = t
	}
	return nil
}

// tls wraps a connection to the phone in TLS, as this computer's half of
// the pairing.
func (l *lockdown) tls(c net.Conn) (net.Conn, error) {
	certPEM, _ := l.record["HostCertificate"].([]byte)
	keyPEM, _ := l.record["HostPrivateKey"].([]byte)
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("the pairing with the iPhone is damaged: %w", err)
	}
	t := tls.Client(c, &tls.Config{
		Certificates: []tls.Certificate{cert},
		// The phone's certificate is the one made when they paired,
		// signed by this computer's own root; nothing to check it
		// against but the pairing.
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS10,
	})
	if err := t.Handshake(); err != nil {
		return nil, fmt.Errorf("starting an encrypted session with the iPhone: %w", err)
	}
	return t, nil
}

// startService starts a service on the phone and connects to it.
func (l *lockdown) startService(name string, escrow bool) (*conn, error) {
	f := map[string]any{"Service": name}
	if bag, ok := l.record["EscrowBag"].([]byte); ok && escrow {
		// Lets the service read files the phone keeps locked while it is.
		f["EscrowBag"] = bag
	}
	d, err := l.request("StartService", f)
	if err != nil {
		return nil, fmt.Errorf("starting %s: %w", name, err)
	}
	port := uint16(plist.Int(d["Port"]))
	c, err := connect(l.dev.id, port)
	if err != nil {
		return nil, err
	}
	if b, _ := d["EnableServiceSSL"].(bool); b {
		t, err := l.tls(c)
		if err != nil {
			c.Close()
			return nil, err
		}
		c = t
	}
	return &conn{c}, nil
}
