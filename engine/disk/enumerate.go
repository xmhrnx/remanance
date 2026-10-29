package disk

import (
	"sort"
	"strings"
	"sync"
)

// Provider is an extra source of devices, such as Android's USB host or
// the privileged helper on Linux and macOS.
type Provider interface {
	List() ([]Info, error)
	Open(Info) (Device, error)
	Handles(id string) bool
}

var (
	providersMu sync.Mutex
	providers   []Provider
)

// Register adds a device provider; its devices appear in List.
func Register(p Provider) {
	providersMu.Lock()
	defer providersMu.Unlock()
	providers = append(providers, p)
}

// List returns the drives this platform can see, the system drive last.
func List() ([]Info, error) {
	out, err := listPhysical()
	providersMu.Lock()
	ps := append([]Provider(nil), providers...)
	providersMu.Unlock()
	for _, p := range ps {
		more, perr := p.List()
		if perr == nil {
			out = mergeInfos(out, more)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].System != out[j].System {
			return !out[i].System
		}
		return out[i].Removable && !out[j].Removable
	})
	return out, err
}

func mergeInfos(a, b []Info) []Info {
	seen := map[string]bool{}
	for _, i := range a {
		seen[i.ID] = true
	}
	for _, i := range b {
		if !seen[i.ID] {
			a = append(a, i)
		}
	}
	return a
}

// Open opens a device listed by List, or an image by its "image:" ID.
func Open(info Info) (Device, error) {
	if strings.HasPrefix(info.ID, "image:") {
		return OpenImage(strings.TrimPrefix(info.ID, "image:"))
	}
	providersMu.Lock()
	ps := append([]Provider(nil), providers...)
	providersMu.Unlock()
	for _, p := range ps {
		if p.Handles(info.ID) {
			return p.Open(info)
		}
	}
	return openPhysical(info)
}
