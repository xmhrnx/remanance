//go:build linux

package disk

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func sysRead(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// linuxMounts maps a block device name (sda, nvme0n1) to the mount points
// of it and its partitions, and a partition's name (sda1) to its own.
func linuxMounts() map[string][]string {
	out := map[string][]string{}
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 || !strings.HasPrefix(fields[0], "/dev/") {
			continue
		}
		part := filepath.Base(fields[0])
		// Find the whole disk the partition belongs to through sysfs.
		disk := part
		if link, err := filepath.EvalSymlinks("/sys/class/block/" + part); err == nil {
			if _, err := os.Stat(filepath.Join(link, "partition")); err == nil {
				disk = filepath.Base(filepath.Dir(link))
			}
		}
		out[disk] = append(out[disk], fields[1])
		if disk != part {
			out[part] = append(out[part], fields[1])
		}
	}
	return out
}

// labels maps partition names to their file systems' labels, which udev
// links in /dev/disk/by-label.
func labels() map[string]string {
	out := map[string]string{}
	es, _ := os.ReadDir("/dev/disk/by-label")
	for _, e := range es {
		if dev, err := filepath.EvalSymlinks("/dev/disk/by-label/" + e.Name()); err == nil {
			out[filepath.Base(dev)] = strings.ReplaceAll(e.Name(), `\x20`, " ")
		}
	}
	return out
}

// partitions lists a disk's partitions, so one can be scanned on its
// own.
func partitions(disk string, of Info, mounts map[string][]string, names map[string]string) []Info {
	es, _ := os.ReadDir("/sys/block/" + disk)
	var out []Info
	for _, e := range es {
		base := "/sys/block/" + disk + "/" + e.Name()
		if _, err := os.Stat(base + "/partition"); err != nil {
			continue
		}
		sectors, _ := strconv.ParseInt(sysRead(base+"/size"), 10, 64)
		if sectors < 2048 {
			continue // an extended partition's stub
		}
		p := Info{
			ID: "/dev/" + e.Name(), Path: "/dev/" + e.Name(), Kind: KindVolume,
			Name: names[e.Name()], Model: of.Model, Serial: of.Serial, Bus: of.Bus, Removable: of.Removable,
			Size: sectors * 512, SectorSize: of.SectorSize, Mounts: mounts[e.Name()],
		}
		if p.Name == "" {
			p.Name = e.Name()
		}
		for _, m := range p.Mounts {
			if m == "/" {
				p.System = true
			}
		}
		out = append(out, p)
	}
	return out
}

func listPhysical() ([]Info, error) {
	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		return nil, err
	}
	mounts, names := linuxMounts(), labels()
	var out []Info
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "ram") || strings.HasPrefix(name, "zram") || strings.HasPrefix(name, "dm-") {
			continue
		}
		base := "/sys/block/" + name
		sectors, _ := strconv.ParseInt(sysRead(base+"/size"), 10, 64)
		if sectors == 0 {
			continue
		}
		info := Info{
			ID:         "/dev/" + name,
			Path:       "/dev/" + name,
			Kind:       KindPhysical,
			Size:       sectors * 512, // sysfs always counts 512-byte units
			SectorSize: 512,
			Removable:  sysRead(base+"/removable") == "1",
			Mounts:     mounts[name],
		}
		if ss, _ := strconv.Atoi(sysRead(base + "/queue/logical_block_size")); ss > 0 {
			info.SectorSize = ss
		}
		info.Model = strings.TrimSpace(sysRead(base+"/device/vendor") + " " + sysRead(base+"/device/model"))
		info.Serial = sysRead(base + "/device/serial")
		link, _ := filepath.EvalSymlinks(base)
		switch {
		case strings.HasPrefix(name, "loop"):
			info.Bus = "Loop"
			if back := sysRead(base + "/loop/backing_file"); back != "" {
				info.Model = filepath.Base(back)
			}
		case strings.Contains(link, "/usb"):
			info.Bus = "USB"
			info.Removable = true
		case strings.HasPrefix(name, "nvme"):
			info.Bus = "NVMe"
		case strings.HasPrefix(name, "mmcblk"):
			info.Bus = "SD"
			info.Removable = true
		default:
			info.Bus = "SATA"
		}
		for _, m := range info.Mounts {
			if m == "/" {
				info.System = true
			}
		}
		info.Name = info.Model
		if info.Name == "" {
			info.Name = name
		}
		out = append(out, info)
		out = append(out, partitions(name, info, mounts, names)...)
	}
	return out, nil
}
