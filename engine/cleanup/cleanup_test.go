package cleanup

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func write(t *testing.T, path string, size int, age time.Duration) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
	when := time.Now().Add(-age)
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatal(err)
	}
}

func TestFind(t *testing.T) {
	dir := t.TempDir()
	day := 24 * time.Hour
	tmp, cache, dl := filepath.Join(dir, "tmp"), filepath.Join(dir, "cache"), filepath.Join(dir, "dl")
	write(t, filepath.Join(tmp, "old.log"), 100, 3*day)
	write(t, filepath.Join(tmp, "fresh.lock"), 50, time.Minute) // may be in use
	write(t, filepath.Join(tmp, "job", "a"), 10, 5*day)
	write(t, filepath.Join(tmp, "job", "b"), 20, time.Hour) // keeps job as in use
	write(t, filepath.Join(cache, "app", "x"), 300, 0)
	write(t, filepath.Join(cache, "go-build", "y"), 400, 0)
	write(t, filepath.Join(dl, "setup.exe"), 500, 0)
	write(t, filepath.Join(dl, "report.pdf"), 600, 200*day)
	write(t, filepath.Join(dl, "new.pdf"), 700, day)
	sources := []Source{
		{Kind: Temp, Dir: tmp, Each: true},
		{Kind: Caches, Dir: cache, Each: true},
		{Kind: OldDownloads, Dir: dl, Each: true},
	}
	groups, err := FindIn(context.Background(), sources, time.Now(), nil)
	if err != nil {
		t.Fatal(err)
	}
	got := map[Kind][]string{}
	for _, g := range groups {
		for _, it := range g.Items {
			got[g.Kind] = append(got[g.Kind], it.Title())
		}
	}
	want := map[Kind][]string{
		Temp:         {"old.log"},
		Caches:       {"app"},
		BuildCaches:  {"go-build"},
		Installers:   {"setup.exe"},
		OldDownloads: {"report.pdf"},
	}
	for k, w := range want {
		if len(got[k]) != len(w) || got[k][0] != w[0] {
			t.Errorf("%s: got %v, want %v", k.Label(), got[k], w)
		}
	}
	if len(got) != len(want) {
		t.Errorf("got kinds %v", got)
	}
}

func TestClean(t *testing.T) {
	dir := t.TempDir()
	cache := filepath.Join(dir, "cache")
	write(t, filepath.Join(cache, "a", "b", "c"), 100, 0)
	write(t, filepath.Join(cache, "d"), 50, 0)
	single := filepath.Join(dir, "one.log")
	write(t, single, 30, 0)
	items := []Item{
		{Kind: Caches, Path: cache, Size: 150, Files: 2, Keep: true},
		{Kind: Logs, Path: single, Size: 30, Files: 1},
	}
	r, err := Clean(context.Background(), items, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Freed != 180 || r.Files != 3 || r.Left != 0 {
		t.Errorf("result %+v", r)
	}
	es, err := os.ReadDir(cache)
	if err != nil {
		t.Fatalf("the kept folder went too: %v", err)
	}
	if len(es) != 0 {
		t.Errorf("left in the cache: %v", es)
	}
	if _, err := os.Lstat(single); !os.IsNotExist(err) {
		t.Errorf("the log is still there")
	}
}

func TestFree(t *testing.T) {
	s, err := Free(t.TempDir())
	if err != nil {
		t.Skip(err)
	}
	if s.Total <= 0 || s.Free < 0 || s.Free > s.Total {
		t.Errorf("space %+v", s)
	}
}
