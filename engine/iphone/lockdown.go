package iphone

import (
	"errors"
	"net"
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

// lockdown is a session with the phone's lockdown service.
type lockdown struct {
	*conn
	dev    device
	record map[string]any // the pairing, once paired
}

// lockdownError is one of the errors the phone's lockdown service
// replies with.
type lockdownError string

func openLockdown(dev device) (*lockdown, error) {
	c, err := connect(dev.id, lockdownPort)
	if err != nil {
		return nil, err
	}
	return &lockdown{conn: &conn{c}, dev: dev}, nil
}

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
