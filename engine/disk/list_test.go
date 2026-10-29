package disk

import "testing"

func TestListDoesNotFail(t *testing.T) {
	infos, err := List()
	if err != nil {
		t.Fatal(err)
	}
	for _, i := range infos {
		t.Logf("%s %q bus=%s size=%d sector=%d removable=%v system=%v mounts=%v", i.Path, i.Name, i.Bus, i.Size, i.SectorSize, i.Removable, i.System, i.Mounts)
	}
}
