//go:build linux && !android

package disk

import (
	"bufio"
	"errors"
	"io/fs"
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
// of it and its partitions.
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
	}
	return out
}

func listPhysical() ([]Info, error) {
	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		return nil, err
	}
	mounts := linuxMounts()
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
	}
	return out, nil
}

type fileDevice struct {
	f    *os.File
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
	return &fileDevice{f: f, info: info}, nil
}

func (d *fileDevice) ReadAt(p []byte, off int64) (int, error) { return d.f.ReadAt(p, off) }
func (d *fileDevice) Info() Info                              { return d.info }
func (d *fileDevice) Size() int64                             { return d.info.Size }
func (d *fileDevice) SectorSize() int                         { return d.info.SectorSize }
func (d *fileDevice) Close() error                            { return d.f.Close() }
