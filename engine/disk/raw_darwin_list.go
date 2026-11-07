//go:build darwin

package disk

import (
	"encoding/xml"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// plistDict reads the top-level <dict> of a property list into strings.
func plistDict(data []byte) map[string]string {
	dec := xml.NewDecoder(strings.NewReader(string(data)))
	out := map[string]string{}
	depth := 0
	var key string
	for {
		tok, err := dec.Token()
		if err != nil {
			return out
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if depth != 3 { // plist > dict > value
				continue
			}
			var s string
			switch t.Name.Local {
			case "key", "string", "integer":
				dec.DecodeElement(&s, &t)
				depth--
			case "true", "false":
				s = t.Name.Local
			default:
				continue
			}
			if t.Name.Local == "key" {
				key = s
			} else if key != "" {
				out[key] = s
				key = ""
			}
		case xml.EndElement:
			depth--
		}
	}
}

func listPhysical() ([]Info, error) {
	names, _ := filepath.Glob("/dev/disk*")
	var out []Info
	for _, p := range names {
		name := filepath.Base(p)
		if !wholeDisk.MatchString(name) {
			continue
		}
		raw, err := exec.Command("diskutil", "info", "-plist", name).Output()
		if err != nil {
			continue
		}
		d := plistDict(raw)
		size, _ := strconv.ParseInt(d["TotalSize"], 10, 64)
		if size == 0 {
			size, _ = strconv.ParseInt(d["Size"], 10, 64)
		}
		if size == 0 || d["VirtualOrPhysical"] == "Virtual" && d["Content"] == "" {
			continue
		}
		sector, _ := strconv.Atoi(d["DeviceBlockSize"])
		if sector == 0 {
			sector = 512
		}
		info := Info{
			ID:         "/dev/r" + name,
			Path:       "/dev/r" + name,
			Kind:       KindPhysical,
			Name:       d["MediaName"],
			Model:      d["MediaName"],
			Bus:        d["BusProtocol"],
			Removable:  d["Removable"] == "true" || d["RemovableMedia"] == "true" || d["Ejectable"] == "true",
			System:     d["SystemImage"] == "true",
			Size:       size,
			SectorSize: sector,
		}
		if info.Name == "" {
			info.Name = name
		}
		if d["Internal"] == "true" && !info.Removable {
			// The boot disk's APFS container lives on a synthesized disk;
			// mark the physical one that carries it.
			if mp := d["MountPoint"]; mp == "/" {
				info.System = true
			}
		}
		out = append(out, info)
	}
	return append(out, listVolumes(names)...), nil
}

// listVolumes lists the partitions of the disks named, so one can be
// scanned on its own.
func listVolumes(names []string) []Info {
	var out []Info
	for _, p := range names {
		name := filepath.Base(p)
		if !slice.MatchString(name) {
			continue
		}
		raw, err := exec.Command("diskutil", "info", "-plist", name).Output()
		if err != nil {
			continue
		}
		d := plistDict(raw)
		if !readable[d["FilesystemType"]] {
			continue
		}
		size, _ := strconv.ParseInt(d["Size"], 10, 64)
		if size == 0 {
			size, _ = strconv.ParseInt(d["TotalSize"], 10, 64)
		}
		sector, _ := strconv.Atoi(d["DeviceBlockSize"])
		if sector == 0 {
			sector = 512
		}
		info := Info{
			ID: "/dev/r" + name, Path: "/dev/r" + name, Kind: KindVolume,
			Name: d["VolumeName"], Model: d["MediaName"], Bus: d["BusProtocol"],
			Removable: d["Removable"] == "true" || d["RemovableMedia"] == "true" || d["Ejectable"] == "true",
			Size:      size, SectorSize: sector,
		}
		if mp := d["MountPoint"]; mp != "" {
			info.Mounts = []string{mp}
		}
		if info.Name == "" {
			info.Name = name
		}
		out = append(out, info)
	}
	return out
}
