// Package devsync keeps folders the same on your own devices, directly
// between them over the local network, with no cloud and no account.
//
// Each device has a key pair; its ID is a hash of its certificate, and
// devices talk only over TLS to the IDs their owner added, so nothing
// else can join. Devices on the same network find each other by a
// broadcast every half minute; one elsewhere is reached by its address.
//
// A folder is shared by its ID: the device that shares it offers it, and
// the other accepts it into a folder of its own. Each device keeps an
// index of the folder's files, with a hash of each and a version vector,
// a counter per device that changed it. A file whose version on another
// device is newer is fetched in blocks, checked against its hash and
// moved into place; one changed on both since they last agreed is a
// conflict, and the older change is kept beside it as a copy named
// "name.sync-conflict-<date>-<device>.ext", so nothing is lost.
//
// A folder can also back up: "Send only" on the device whose files
// count, "Receive only" on the backup, which can keep the versions a
// change replaced in .datadoc-versions.
package devsync

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
)

// Mode is how a folder syncs on this device.
type Mode int

const (
	TwoWay      Mode = iota // changes go both ways
	SendOnly                // this device's changes go out; others' aren't taken
	ReceiveOnly             // others' changes come in; this device's aren't sent
)

// ModeLabels name the modes, in order.
var ModeLabels = []string{"Two-way", "Send only", "Receive only"}

// Device is another device of the user's.
type Device struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Addr is where to reach it, "host:port", when it isn't found on
	// the local network.
	Addr string `json:"addr,omitempty"`
}

// Folder is a folder kept in sync.
type Folder struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Path  string `json:"path"`
	// Devices are the IDs of the devices it's shared with.
	Devices []string `json:"devices"`
	Mode    Mode     `json:"mode"`
	// Versions keeps what a change from another device replaced or
	// deleted, in .datadoc-versions in the folder.
	Versions bool `json:"versions"`
	Paused   bool `json:"paused"`
}

// Pending is a device that asked to sync with this one, or a folder a
// device offered.
type Pending struct {
	Device string `json:"device"`
	Name   string `json:"name"`
	Addr   string `json:"addr,omitempty"`
	// Folder and Label are set for an offered folder.
	Folder string `json:"folder,omitempty"`
	Label  string `json:"label,omitempty"`
}

// Config is what's kept between runs.
type Config struct {
	Name    string    `json:"name"`
	Devices []Device  `json:"devices"`
	Folders []Folder  `json:"folders"`
	Pending []Pending `json:"pending,omitempty"`
	// Port is where this device listens; 0 picks DefaultPort.
	Port int `json:"port,omitempty"`
}

// DefaultPort is where devices listen for each other, on TCP, and
// announce themselves, on UDP.
const DefaultPort = 22100

func loadConfig(dir string) (Config, error) {
	var c Config
	b, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	return c, json.Unmarshal(b, &c)
}

func saveJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (c *Config) device(id string) *Device {
	for i := range c.Devices {
		if c.Devices[i].ID == id {
			return &c.Devices[i]
		}
	}
	return nil
}

func (c *Config) folder(id string) *Folder {
	for i := range c.Folders {
		if c.Folders[i].ID == id {
			return &c.Folders[i]
		}
	}
	return nil
}

func (f *Folder) sharedWith(id string) bool { return slices.Contains(f.Devices, id) }
