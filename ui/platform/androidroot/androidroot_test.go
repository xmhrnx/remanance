package androidroot

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/xmhrnx/remanance/engine/disk"
)

func TestParse(t *testing.T) {
	out := `major minor  #blocks  name

 259        0  250059776 sda
 259       34  230000000 sda34
 179        0   62521344 mmcblk0
 179        1   62520320 mmcblk0p1
 179        8       4096 mmcblk0rpmb
 254        0    2097152 dm-0
==
userdata /dev/block/sda34
boot_a /dev/block/sda5
`
	got := parse(out)
	if len(got) != 2 {
		t.Fatalf("%+v", got)
	}
	if got[0].Path != "/dev/block/sda34" || got[0].Size != 230000000*1024 || got[0].Name != "Phone storage" {
		t.Fatalf("userdata: %+v", got[0])
	}
	if got[1].Path != "/dev/block/mmcblk0" || !got[1].Removable {
		t.Fatalf("card: %+v", got[1])
	}
	// On an eMMC phone mmcblk0 is the phone's own chip, not a card.
	got = parse("179 0 62521344 mmcblk0\n179 40 50000000 mmcblk0p40\n==\nuserdata /dev/block/mmcblk0p40\n")
	if len(got) != 1 || got[0].Path != "/dev/block/mmcblk0p40" {
		t.Fatalf("emmc: %+v", got)
	}
}

// TestRead runs the shell side with sh and dd in place of su, reading a
// file as the device.
func TestRead(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	if _, err := exec.LookPath("dd"); err != nil {
		t.Skip("no dd")
	}
	data := make([]byte, 64<<10)
	for i := range data {
		data[i] = byte(i * 13)
	}
	dev := filepath.Join(t.TempDir(), "dev")
	os.WriteFile(dev, data, 0o644)
	// sh isn't root, so it goes in without start's check.
	p := &provider{start: func() (*session, error) { return spawn("sh") }}
	d, err := p.Open(disk.Info{ID: prefix + dev, Path: filepath.ToSlash(dev), Size: int64(len(data))})
	if err != nil {
		t.Fatal(err)
	}
	got := make([]byte, 10000)
	if _, err := d.ReadAt(got, 1234); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data[1234:11234]) {
		t.Fatal("read the wrong bytes")
	}
	p.shell.close()
}
