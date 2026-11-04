package locales

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestKeysExist checks every key the app asks for has an English message,
// so no screen shows a bare key.
func TestKeysExist(t *testing.T) {
	b, err := os.ReadFile("en.json")
	if err != nil {
		t.Fatal(err)
	}
	var en map[string]any
	if err := json.Unmarshal(b, &en); err != nil {
		t.Fatal(err)
	}
	call := regexp.MustCompile(`locales\.[TN]\("([^"]+)"`)
	noun := regexp.MustCompile(`count\([^\n]*?, "([a-z]+)"\)`)
	err = filepath.WalkDir("../ui", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") {
			return err
		}
		src, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for _, m := range call.FindAllStringSubmatch(string(src), -1) {
			if strings.HasSuffix(m[1], ".") {
				continue // a prefix the code adds a name to
			}
			if _, ok := en[m[1]]; !ok {
				t.Errorf("%s: no message for %q", p, m[1])
			}
		}
		for _, m := range noun.FindAllStringSubmatch(string(src), -1) {
			if _, ok := en["count."+m[1]]; !ok {
				t.Errorf("%s: no message for count.%s", p, m[1])
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestMessages(t *testing.T) {
	if got := T("results.recover_n", "n", 3); got != "Recover 3" {
		t.Errorf("T = %q", got)
	}
	if got := N("count.file", 1); got != "1 file" {
		t.Errorf("N(1) = %q", got)
	}
	if got := N("count.file", 4); got != "4 files" {
		t.Errorf("N(4) = %q", got)
	}
	if got := Label("Exact copies"); got != "Exact copies" {
		t.Errorf("Label = %q", got)
	}
	if got := Label("Not a label"); got != "Not a label" {
		t.Errorf("Label of an unknown name = %q", got)
	}
}
