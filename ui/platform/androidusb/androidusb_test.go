package androidusb

import (
	"os"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	got := parse("/dev/bus/usb/001/002\t2385\t5734\tKingston\tDataTraveler 3.0\t0\n/dev/bus/usb/001/003\t1234\t43981\t\t\t1\n")
	if len(got) != 2 {
		t.Fatalf("%d drives", len(got))
	}
	if got[0].ID != "usb:/dev/bus/usb/001/002" || got[0].Name != "Kingston DataTraveler 3.0" {
		t.Fatalf("first: %+v", got[0])
	}
	if got[1].Name != "USB drive 4d2:abcd" {
		t.Fatalf("unnamed: %q", got[1].Name)
	}
	if len(parse("")) != 0 {
		t.Fatal("empty list")
	}
}

func TestParseOpen(t *testing.T) {
	fd, iface, in, out, err := parseOpen("57 0 129 2")
	if err != nil || fd != 57 || iface != 0 || in != 129 || out != 2 {
		t.Fatalf("%d %d %d %d %v", fd, iface, in, out, err)
	}
	if _, _, _, _, err := parseOpen("error: access to the drive wasn't allowed"); err == nil || err.Error() != "access to the drive wasn't allowed" {
		t.Fatalf("error answer: %v", err)
	}
	if _, _, _, _, err := parseOpen("garbage"); err == nil {
		t.Fatal("garbage accepted")
	}
}

func TestBridgeClass(t *testing.T) {
	src, err := os.ReadFile("UsbBridge.java")
	if err != nil {
		t.Fatal(err)
	}
	pkg := strings.ReplaceAll(strings.TrimSuffix(bridgeClass, "/UsbBridge"), "/", ".")
	if !strings.Contains(string(src), "package "+pkg+";") {
		t.Fatalf("UsbBridge.java isn't in package %s, which bridgeClass names", pkg)
	}
}
