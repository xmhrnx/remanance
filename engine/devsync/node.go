package devsync

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Node is this device: its folders, the devices it syncs with and its
// connections to them.
type Node struct {
	dir  string
	cert tls.Certificate
	id   string

	mu     sync.Mutex
	cfg    Config
	idx    map[string]*index                     // folder ID → this device's files
	remote map[string]map[string]map[string]File // folder → device → name → file
	conns  map[string]*conn
	nearby map[string]Nearby
	state  map[string]*folderState
	change func()

	ctx    context.Context
	cancel context.CancelFunc
	ln     net.Listener
	wake   chan struct{} // scan and pull now
	dial   chan struct{} // connect now
	wg     sync.WaitGroup
}

// Nearby is a device found on the local network.
type Nearby struct {
	ID, Name, Addr string
	Seen           time.Time
}

type folderState struct {
	scanning, pulling bool
	err               string
	lastSync          time.Time
	pullNeeded        bool
}

// Open loads this device's settings and identity from dir, making them
// the first time; name is what other devices call it until it's changed.
func Open(dir, name string) (*Node, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	cert, err := identity(dir)
	if err != nil {
		return nil, fmt.Errorf("making this device's key: %w", err)
	}
	cfg, err := loadConfig(dir)
	if err != nil {
		return nil, err
	}
	if cfg.Name == "" {
		cfg.Name = name
	}
	n := &Node{dir: dir, cert: cert, id: idOf(cert.Certificate[0]), cfg: cfg,
		idx: map[string]*index{}, remote: map[string]map[string]map[string]File{}, conns: map[string]*conn{},
		nearby: map[string]Nearby{}, state: map[string]*folderState{},
		wake: make(chan struct{}, 1), dial: make(chan struct{}, 1)}
	for _, f := range cfg.Folders {
		n.idx[f.ID] = loadIndex(n.indexPath(f.ID))
		n.state[f.ID] = &folderState{}
	}
	return n, nil
}

// ID is this device's ID, for the user to give their other devices.
func (n *Node) ID() string { return n.id }

func (n *Node) indexPath(folder string) string { return filepath.Join(n.dir, "index-"+folder+".json") }

// OnChange sets what's called when anything shown changes: a device
// connects, a folder syncs, a device asks to join.
func (n *Node) OnChange(fn func()) {
	n.mu.Lock()
	n.change = fn
	n.mu.Unlock()
}

func (n *Node) changed() {
	n.mu.Lock()
	fn := n.change
	n.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// Options say how to run.
type Options struct {
	// Port to listen on; 0 is the configured one, or DefaultPort, and -1
	// any free port.
	Port int
	// Discovery announces this device on the local network and listens
	// for others.
	Discovery bool
	// Rescan is how often folders are checked for changes; a minute by
	// default.
	Rescan time.Duration
}

// Start listens for the other devices, finds and connects to them, and
// keeps the folders in sync until ctx ends or Close.
func (n *Node) Start(ctx context.Context, o Options) error {
	port := o.Port
	if port == 0 {
		port = n.cfg.Port
	}
	if port == 0 {
		port = DefaultPort
	}
	if port < 0 {
		port = 0
	}
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil && port != 0 {
		// Taken, perhaps by another copy: any port, found by discovery.
		ln, err = net.Listen("tcp", ":0")
	}
	if err != nil {
		return err
	}
	n.ctx, n.cancel = context.WithCancel(ctx)
	n.ln = ln
	if o.Rescan == 0 {
		o.Rescan = time.Minute
	}
	n.wg.Add(3)
	go n.acceptLoop()
	go n.dialLoop()
	go n.syncLoop(o.Rescan)
	if o.Discovery {
		n.wg.Add(1)
		go n.discover(port)
	}
	return nil
}

// Addr is where this device listens.
func (n *Node) Addr() string {
	if n.ln == nil {
		return ""
	}
	return n.ln.Addr().String()
}

// Close stops syncing and disconnects.
func (n *Node) Close() error {
	if n.cancel == nil {
		return nil
	}
	n.cancel()
	n.ln.Close()
	n.mu.Lock()
	for _, c := range n.conns {
		c.close()
	}
	n.mu.Unlock()
	n.wg.Wait()
	return nil
}

// SyncNow checks every folder for changes, and connects to devices not
// connected, now instead of at the next minute.
func (n *Node) SyncNow() {
	select {
	case n.wake <- struct{}{}:
	default:
	}
	select {
	case n.dial <- struct{}{}:
	default:
	}
}

func (n *Node) tlsConfig() *tls.Config {
	return &tls.Config{
		Certificates:       []tls.Certificate{n.cert},
		ClientAuth:         tls.RequireAnyClientCert,
		InsecureSkipVerify: true, // a device is known by its certificate's hash, checked by the caller
		MinVersion:         tls.VersionTLS13,
	}
}

func (n *Node) acceptLoop() {
	defer n.wg.Done()
	for {
		raw, err := n.ln.Accept()
		if err != nil {
			return
		}
		go func() {
			t := tls.Server(raw, n.tlsConfig())
			t.SetDeadline(time.Now().Add(20 * time.Second))
			if err := t.Handshake(); err != nil {
				t.Close()
				return
			}
			certs := t.ConnectionState().PeerCertificates
			if len(certs) == 0 {
				t.Close()
				return
			}
			n.serve(newConn(t, idOf(certs[0].Raw), true))
		}()
	}
}

func (n *Node) dialLoop() {
	defer n.wg.Done()
	t := time.NewTicker(20 * time.Second)
	defer t.Stop()
	for {
		n.mu.Lock()
		var todo []Device
		for _, d := range n.cfg.Devices {
			if n.conns[d.ID] != nil {
				continue
			}
			if nb, ok := n.nearby[d.ID]; ok && d.Addr == "" {
				d.Addr = nb.Addr
			}
			if d.Addr != "" {
				todo = append(todo, d)
			}
		}
		n.mu.Unlock()
		for _, d := range todo {
			go n.connect(d)
		}
		select {
		case <-n.ctx.Done():
			return
		case <-t.C:
		case <-n.dial:
		}
	}
}

// connect dials a device and checks it is the one asked for.
func (n *Node) connect(d Device) {
	cfg := n.tlsConfig()
	cfg.VerifyPeerCertificate = func(raw [][]byte, _ [][]*x509.Certificate) error {
		if len(raw) == 0 || idOf(raw[0]) != d.ID {
			return errors.New("a different device answered")
		}
		return nil
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	t, err := tls.DialWithDialer(dialer, "tcp", d.Addr, cfg)
	if err != nil {
		return
	}
	n.serve(newConn(t, d.ID, false))
}

// serve says hello, then handles what the device sends until it goes.
func (n *Node) serve(c *conn) {
	c.tls.SetDeadline(time.Now().Add(20 * time.Second))
	if err := c.sendJSON(msgHello, n.helloFor(c.device)); err != nil {
		c.close()
		return
	}
	typ, body, err := c.read()
	var h hello
	if err != nil || typ != msgHello || json.Unmarshal(body, &h) != nil {
		c.close()
		return
	}
	c.tls.SetDeadline(time.Time{})
	c.name = h.Name

	n.mu.Lock()
	if n.cfg.device(c.device) == nil {
		// Not one of the user's devices yet: ask them.
		n.addPending(Pending{Device: c.device, Name: h.Name, Addr: hostPort(c)})
		n.mu.Unlock()
		c.close()
		n.changed()
		return
	}
	if old := n.conns[c.device]; old != nil {
		// Both dialed at once: keep the connection the device with the
		// smaller ID dialed, as the other side does.
		keepNew := (n.id < c.device) != c.inbound
		if !keepNew {
			n.mu.Unlock()
			c.close()
			return
		}
		old.close()
	}
	n.conns[c.device] = c
	n.applyHello(c, h)
	n.mu.Unlock()
	n.changed()
	n.sendIndexes(c)

	for {
		typ, body, err := c.read()
		if err != nil {
			break
		}
		switch typ {
		case msgHello:
			if json.Unmarshal(body, &h) == nil {
				n.mu.Lock()
				n.applyHello(c, h)
				n.mu.Unlock()
				n.changed()
				n.sendIndexes(c)
			}
		case msgIndex:
			var m indexMsg
			if json.Unmarshal(body, &m) == nil {
				n.receiveIndex(c, m)
			}
		case msgRequest:
			var r request
			if json.Unmarshal(body, &r) == nil {
				go n.serveRequest(c, r)
			}
		case msgResponse:
			c.delivered(body)
		}
	}
	c.close()
	n.mu.Lock()
	if n.conns[c.device] == c {
		delete(n.conns, c.device)
		for _, byDev := range n.remote {
			delete(byDev, c.device)
		}
	}
	n.mu.Unlock()
	n.changed()
}

func hostPort(c *conn) string {
	host, _, err := net.SplitHostPort(c.addr)
	if err != nil || c.inbound {
		// An inbound connection comes from some port; its listening
		// port is learned by discovery.
		return ""
	}
	return net.JoinHostPort(host, strconv.Itoa(DefaultPort))
}

// helloFor lists the folders shared with a device.
func (n *Node) helloFor(device string) hello {
	n.mu.Lock()
	defer n.mu.Unlock()
	h := hello{Name: n.cfg.Name}
	for _, f := range n.cfg.Folders {
		if f.sharedWith(device) {
			h.Folders = append(h.Folders, folderOffer{ID: f.ID, Label: f.Label})
		}
	}
	return h
}

// applyHello records which folders a device shares, and offers the
// user the ones not here yet. n.mu is held.
func (n *Node) applyHello(c *conn, h hello) {
	c.folders = map[string]bool{}
	if d := n.cfg.device(c.device); d != nil && h.Name != "" && d.Name != h.Name {
		d.Name = h.Name
		n.save()
	}
	for _, f := range h.Folders {
		c.folders[f.ID] = true
		local := n.cfg.folder(f.ID)
		switch {
		case local == nil:
			n.addPending(Pending{Device: c.device, Name: h.Name, Folder: f.ID, Label: f.Label})
		case !local.sharedWith(c.device):
			// Shared here too, under another device: offer adding it.
			n.addPending(Pending{Device: c.device, Name: h.Name, Folder: f.ID, Label: f.Label})
		}
	}
}

// addPending records a request once. n.mu is held.
func (n *Node) addPending(p Pending) {
	for _, q := range n.cfg.Pending {
		if q.Device == p.Device && q.Folder == p.Folder {
			return
		}
	}
	n.cfg.Pending = append(n.cfg.Pending, p)
	n.save()
}

// shared tells whether a folder is shared both ways with a connection's
// device. n.mu is held.
func (n *Node) shared(folder string, c *conn) *Folder {
	f := n.cfg.folder(folder)
	if f == nil || !f.sharedWith(c.device) || !c.folders[folder] {
		return nil
	}
	return f
}

// sendIndexes sends a device every file of the folders it shares.
func (n *Node) sendIndexes(c *conn) {
	n.mu.Lock()
	var msgs []indexMsg
	for _, f := range n.cfg.Folders {
		if n.shared(f.ID, c) == nil {
			continue
		}
		m := indexMsg{Folder: f.ID, Full: true}
		if f.Mode != ReceiveOnly {
			for _, file := range n.idx[f.ID].Files {
				m.Files = append(m.Files, file)
			}
		}
		msgs = append(msgs, m)
	}
	n.mu.Unlock()
	for _, m := range msgs {
		c.sendJSON(msgIndex, m)
	}
}

// announce sends the devices sharing a folder the files that changed.
func (n *Node) announce(folder string, files []File) {
	if len(files) == 0 {
		return
	}
	n.mu.Lock()
	f := n.cfg.folder(folder)
	if f == nil || f.Mode == ReceiveOnly {
		n.mu.Unlock()
		return
	}
	var to []*conn
	for _, c := range n.conns {
		if n.shared(folder, c) != nil {
			to = append(to, c)
		}
	}
	n.mu.Unlock()
	for _, c := range to {
		c.sendJSON(msgIndex, indexMsg{Folder: folder, Files: files})
	}
}

func (n *Node) receiveIndex(c *conn, m indexMsg) {
	n.mu.Lock()
	if n.shared(m.Folder, c) == nil {
		n.mu.Unlock()
		return
	}
	byDev := n.remote[m.Folder]
	if byDev == nil {
		byDev = map[string]map[string]File{}
		n.remote[m.Folder] = byDev
	}
	files := byDev[c.device]
	if files == nil || m.Full {
		files = map[string]File{}
		byDev[c.device] = files
	}
	for _, f := range m.Files {
		if validName(f.Name) {
			files[f.Name] = f
		}
	}
	n.state[m.Folder].pullNeeded = true
	n.mu.Unlock()
	select {
	case n.wake <- struct{}{}:
	default:
	}
}

// serveRequest reads a block of a file for a device, if the file is
// still the version it asks for.
func (n *Node) serveRequest(c *conn, r request) {
	n.mu.Lock()
	f := n.shared(r.Folder, c)
	var root string
	var file File
	if f != nil {
		root = f.Path
		file = n.idx[r.Folder].Files[r.Name]
	}
	n.mu.Unlock()
	switch {
	case f == nil || !validName(r.Name):
		c.respond(r.ID, nil, errors.New("no such folder"))
		return
	case file.Hash != r.Hash || file.Deleted:
		c.respond(r.ID, nil, errors.New("the file changed"))
		return
	case r.Size <= 0 || r.Size > 2*blockSize:
		c.respond(r.ID, nil, errors.New("bad request"))
		return
	}
	fh, err := os.Open(filepath.Join(root, filepath.FromSlash(r.Name)))
	if err != nil {
		c.respond(r.ID, nil, err)
		return
	}
	defer fh.Close()
	buf := make([]byte, r.Size)
	k, err := fh.ReadAt(buf, r.Offset)
	if k < r.Size && (err == nil || k == 0 && r.Offset < file.Size) {
		if err == nil {
			err = errors.New("short read")
		}
		c.respond(r.ID, nil, err)
		return
	}
	c.respond(r.ID, buf[:k], nil)
}

func (n *Node) save() {
	saveJSON(filepath.Join(n.dir, "config.json"), n.cfg)
}

// Config returns a copy of the settings.
func (n *Node) Config() Config {
	n.mu.Lock()
	defer n.mu.Unlock()
	c := n.cfg
	c.Devices = slices.Clone(c.Devices)
	c.Folders = slices.Clone(c.Folders)
	c.Pending = slices.Clone(c.Pending)
	return c
}

// SetName renames this device, as the others see it.
func (n *Node) SetName(name string) {
	n.mu.Lock()
	n.cfg.Name = strings.TrimSpace(name)
	n.save()
	n.mu.Unlock()
	n.rehello()
}

// AddDevice adds one of the user's devices by its ID. It connects at
// once if the device is nearby or addr is given; the other device then
// asks its owner to accept this one.
func (n *Node) AddDevice(id, name, addr string) error {
	id = NormalizeID(id)
	if id == "" {
		return errors.New("that isn't a device ID: it's 32 letters and digits, as the other device shows it")
	}
	if id == n.id {
		return errors.New("that's this device's own ID")
	}
	n.mu.Lock()
	if d := n.cfg.device(id); d != nil {
		if addr != "" {
			d.Addr = addr
		}
	} else {
		if name == "" {
			if nb, ok := n.nearby[id]; ok {
				name = nb.Name
			} else {
				name = "Device " + id[:4]
			}
		}
		n.cfg.Devices = append(n.cfg.Devices, Device{ID: id, Name: name, Addr: addr})
	}
	n.dropPending(id, "")
	n.save()
	n.mu.Unlock()
	n.changed()
	n.SyncNow()
	return nil
}

// RemoveDevice stops syncing with a device.
func (n *Node) RemoveDevice(id string) {
	n.mu.Lock()
	n.cfg.Devices = slices.DeleteFunc(n.cfg.Devices, func(d Device) bool { return d.ID == id })
	for i := range n.cfg.Folders {
		n.cfg.Folders[i].Devices = slices.DeleteFunc(n.cfg.Folders[i].Devices, func(s string) bool { return s == id })
	}
	n.dropPending(id, "")
	n.save()
	c := n.conns[id]
	n.mu.Unlock()
	if c != nil {
		c.close()
	}
	n.changed()
}

// Dismiss forgets a request without accepting it.
func (n *Node) Dismiss(p Pending) {
	n.mu.Lock()
	n.dropPending(p.Device, p.Folder)
	n.save()
	n.mu.Unlock()
	n.changed()
}

// dropPending forgets a device's request, or its offer of a folder; an
// empty folder drops the device's request to join. n.mu is held.
func (n *Node) dropPending(device, folder string) {
	n.cfg.Pending = slices.DeleteFunc(n.cfg.Pending, func(p Pending) bool { return p.Device == device && p.Folder == folder })
}

// AddFolder starts syncing a folder; one with no ID gets a new one, to
// offer the devices it's shared with.
func (n *Node) AddFolder(f Folder) (Folder, error) {
	if f.Path == "" {
		return f, errors.New("choose the folder to sync")
	}
	if err := os.MkdirAll(f.Path, 0o755); err != nil {
		return f, err
	}
	if f.ID == "" {
		f.ID = newFolderID()
	}
	if f.Label == "" {
		f.Label = filepath.Base(f.Path)
	}
	n.mu.Lock()
	for _, g := range n.cfg.Folders {
		if g.ID == f.ID {
			n.mu.Unlock()
			return f, errors.New("that folder is already synced here")
		}
		if sameDir(g.Path, f.Path) {
			n.mu.Unlock()
			return f, fmt.Errorf("%s is already synced, as %q", f.Path, g.Label)
		}
	}
	n.cfg.Folders = append(n.cfg.Folders, f)
	n.idx[f.ID] = loadIndex(n.indexPath(f.ID))
	n.state[f.ID] = &folderState{}
	for _, d := range f.Devices {
		n.dropPending(d, f.ID)
	}
	n.save()
	n.mu.Unlock()
	n.rehello()
	n.changed()
	n.SyncNow()
	return f, nil
}

// AcceptFolder syncs a folder a device offered into path.
func (n *Node) AcceptFolder(p Pending, path string, mode Mode) error {
	n.mu.Lock()
	if f := n.cfg.folder(p.Folder); f != nil {
		// Already here: share it with this device too.
		if !f.sharedWith(p.Device) {
			f.Devices = append(f.Devices, p.Device)
		}
		n.dropPending(p.Device, p.Folder)
		n.save()
		n.mu.Unlock()
		n.rehello()
		n.changed()
		return nil
	}
	n.mu.Unlock()
	_, err := n.AddFolder(Folder{ID: p.Folder, Label: p.Label, Path: path, Devices: []string{p.Device}, Mode: mode, Versions: mode == ReceiveOnly})
	return err
}

// UpdateFolder changes a folder's settings: who it's shared with, its
// mode, versions, pausing.
func (n *Node) UpdateFolder(f Folder) {
	n.mu.Lock()
	if g := n.cfg.folder(f.ID); g != nil {
		f.Path = g.Path // moving a folder isn't a setting
		*g = f
		n.save()
	}
	n.mu.Unlock()
	n.rehello()
	n.changed()
	n.SyncNow()
}

// RemoveFolder stops syncing a folder. Its files stay.
func (n *Node) RemoveFolder(id string) {
	n.mu.Lock()
	n.cfg.Folders = slices.DeleteFunc(n.cfg.Folders, func(f Folder) bool { return f.ID == id })
	delete(n.idx, id)
	delete(n.remote, id)
	delete(n.state, id)
	os.Remove(n.indexPath(id))
	n.save()
	n.mu.Unlock()
	n.rehello()
	n.changed()
}

// rehello tells every connected device what's shared with it now.
func (n *Node) rehello() {
	n.mu.Lock()
	conns := make([]*conn, 0, len(n.conns))
	for _, c := range n.conns {
		conns = append(conns, c)
	}
	n.mu.Unlock()
	for _, c := range conns {
		c.sendJSON(msgHello, n.helloFor(c.device))
	}
}

func newFolderID() string {
	b := make([]byte, 10)
	rand.Read(b)
	s := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b))
	return s[:5] + "-" + s[5:10] + "-" + s[10:]
}

func sameDir(a, b string) bool {
	a, _ = filepath.Abs(a)
	b, _ = filepath.Abs(b)
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

// Nearby lists the devices found on the local network lately that
// aren't added yet.
func (n *Node) Nearby() []Nearby {
	n.mu.Lock()
	defer n.mu.Unlock()
	var out []Nearby
	for _, nb := range n.nearby {
		if n.cfg.device(nb.ID) == nil && time.Since(nb.Seen) < 2*time.Minute {
			out = append(out, nb)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
