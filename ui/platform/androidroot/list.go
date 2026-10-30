package androidroot

import (
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/xmhrnx/remanance/engine/disk"
)

// prefix starts the ID of every device found here.
const prefix = "root:"

// listCommand prints the kernel's block devices, a separator, then each
// partition's name with the device it links to.
const listCommand = `cat /proc/partitions; echo ==; for l in /dev/block/by-name/* /dev/block/bootdevice/by-name/*; do [ -e "$l" ] && echo "$(basename "$l") $(readlink -f "$l")"; done`

// wholeCard matches a memory card or eMMC chip as a whole, not a
// partition of one or its boot and RPMB areas.
var wholeCard = regexp.MustCompile(`^mmcblk\d+$`)

// parse reads listCommand's output into what's worth scanning: the
// userdata partition, and memory cards, the mmcblk devices the phone's
// own partitions aren't on.
func parse(listing string) []disk.Info {
	parts, links, _ := strings.Cut(listing, "==")
	sizes := map[string]int64{}
	for _, line := range strings.Split(parts, "\n") {
		f := strings.Fields(line)
		if len(f) != 4 {
			continue
		}
		blocks, err := strconv.ParseInt(f[2], 10, 64)
		if err != nil {
			continue
		}
		sizes[f[3]] = blocks * 1024
	}
	internal := map[string]bool{} // the devices holding named partitions
	userdata := ""
	for _, line := range strings.Split(links, "\n") {
		label, target, ok := strings.Cut(strings.TrimSpace(line), " ")
		if !ok {
			continue
		}
		dev := path.Base(target)
		internal[disk0(dev)] = true
		if label == "userdata" {
			userdata = dev
		}
	}
	var out []disk.Info
	if size := sizes[userdata]; size > 0 {
		out = append(out, disk.Info{
			ID: prefix + "/dev/block/" + userdata, Path: "/dev/block/" + userdata,
			Name: "Phone storage", Bus: "Internal, with root",
			Kind: disk.KindVolume, System: true, Size: size, SectorSize: 512,
		})
	}
	for name, size := range sizes {
		if !wholeCard.MatchString(name) || internal[name] || size == 0 {
			continue
		}
		out = append(out, disk.Info{
			ID: prefix + "/dev/block/" + name, Path: "/dev/block/" + name,
			Name: "Memory card", Bus: "SD, with root",
			Kind: disk.KindPhysical, Removable: true, Size: size, SectorSize: 512,
		})
	}
	return out
}

// disk0 is the whole device a partition is on: sda for sda34, mmcblk0
// for mmcblk0p12.
func disk0(part string) string {
	if strings.HasPrefix(part, "mmcblk") || strings.HasPrefix(part, "nvme") {
		if i := strings.LastIndex(part, "p"); i > 0 {
			return part[:i]
		}
		return part
	}
	return strings.TrimRight(part, "0123456789")
}
