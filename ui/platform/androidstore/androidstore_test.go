package androidstore

import (
	"os"
	"strings"
	"testing"
)

func TestBridgeClass(t *testing.T) {
	src, err := os.ReadFile("StorageBridge.java")
	if err != nil {
		t.Fatal(err)
	}
	pkg := strings.ReplaceAll(strings.TrimSuffix(bridgeClass, "/StorageBridge"), "/", ".")
	if !strings.Contains(string(src), "package "+pkg+";") {
		t.Fatalf("StorageBridge.java isn't in package %s, which bridgeClass names", pkg)
	}
}
