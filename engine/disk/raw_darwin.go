//go:build darwin && !ios

package disk

import (
	"encoding/xml"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var wholeDisk = regexp.MustCompile(`^disk[0-9]+$`)

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

var slice = regexp.MustCompile(`^disk[0-9]+s[0-9]+$`)

// readable are the file systems, as diskutil names them, whose
// partitions are listed to scan on their own; APFS and HFS+ aren't read,
// so their disks are scanned whole, by signature.
var readable = map[string]bool{"msdos": true, "exfat": true, "ntfs": true}

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

type fileDevice struct {
	f    *os.File
	r    *alignedReader
	info Info
}

func openPhysical(info Info) (Device, error) {
	f, err := os.Open(info.Path)
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			return nil, ErrPermission
		}
		return nil, err
	}
	return &fileDevice{f: f, r: Aligned(f, info.SectorSize, info.Size).(*alignedReader), info: info}, nil
}

func (d *fileDevice) ReadAt(p []byte, off int64) (int, error) { return d.r.ReadAt(p, off) }
func (d *fileDevice) Info() Info                              { return d.info }
func (d *fileDevice) Size() int64                             { return d.info.Size }
func (d *fileDevice) SectorSize() int                         { return d.info.SectorSize }
func (d *fileDevice) Close() error                            { return d.f.Close() }
