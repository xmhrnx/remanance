package devsync

import (
	"encoding/json"
	"net"
	"strconv"
	"time"
)

// announcement is what a device broadcasts on the local network, to the
// same port it listens on, over UDP.
type announcement struct {
	App  string `json:"app"` // "remanence-sync"
	ID   string `json:"id"`
	Name string `json:"name"`
	Port int    `json:"port"`
}

const appTag = "remanence-sync"

// discover announces this device every half minute and records the
// devices that announce themselves.
func (n *Node) discover(port int) {
	defer n.wg.Done()
	if port == 0 {
		port = DefaultPort
	}
	pc, err := net.ListenUDP("udp4", &net.UDPAddr{Port: port})
	if err != nil {
		// Another copy listens: announce only.
		pc, err = net.ListenUDP("udp4", &net.UDPAddr{})
		if err != nil {
			return
		}
	}
	go func() {
		<-n.ctx.Done()
		pc.Close()
	}()
	go n.announceLoop(pc, port)
	buf := make([]byte, 2048)
	for {
		k, from, err := pc.ReadFromUDP(buf)
		if err != nil {
			return
		}
		var a announcement
		if json.Unmarshal(buf[:k], &a) != nil || a.App != appTag || a.ID == n.id || NormalizeID(a.ID) != a.ID || a.Port <= 0 {
			continue
		}
		addr := net.JoinHostPort(from.IP.String(), strconv.Itoa(a.Port))
		n.mu.Lock()
		old, known := n.nearby[a.ID]
		n.nearby[a.ID] = Nearby{ID: a.ID, Name: a.Name, Addr: addr, Seen: time.Now()}
		added := n.cfg.device(a.ID) != nil
		connected := n.conns[a.ID] != nil
		n.mu.Unlock()
		if !known || old.Addr != addr {
			n.changed()
		}
		if added && !connected {
			select {
			case n.dial <- struct{}{}:
			default:
			}
		}
	}
}

func (n *Node) announceLoop(pc *net.UDPConn, port int) {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		_, lport, _ := net.SplitHostPort(n.Addr())
		p, _ := strconv.Atoi(lport)
		n.mu.Lock()
		msg, _ := json.Marshal(announcement{App: appTag, ID: n.id, Name: n.cfg.Name, Port: p})
		n.mu.Unlock()
		for _, dst := range broadcasts() {
			pc.WriteToUDP(msg, &net.UDPAddr{IP: dst, Port: port})
		}
		select {
		case <-n.ctx.Done():
			return
		case <-t.C:
		}
	}
}

// broadcasts are the broadcast addresses of this device's networks.
func broadcasts() []net.IP {
	out := []net.IP{net.IPv4bcast}
	ifs, _ := net.Interfaces()
	for _, ifc := range ifs {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagBroadcast == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok || ipn.IP.To4() == nil {
				continue
			}
			ip, mask := ipn.IP.To4(), ipn.Mask
			if len(mask) == 16 {
				mask = mask[12:]
			}
			b := make(net.IP, 4)
			for i := range b {
				b[i] = ip[i] | ^mask[i]
			}
			out = append(out, b)
		}
	}
	return out
}
