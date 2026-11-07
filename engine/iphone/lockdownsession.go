package iphone

import (
	"crypto/tls"
	"fmt"
	"net"

	"github.com/xmhrnx/remanance/engine/plist"
)

// request sends a request and reads its reply, which says Error if it
// failed.
func (l *lockdown) request(name string, fields map[string]any) (map[string]any, error) {
	msg := map[string]any{"Label": "DataDoc", "Request": name}
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
