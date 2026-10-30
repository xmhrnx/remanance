package plist

import (
	"bytes"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	for _, name := range []string{"binary.plist", "xml.plist"} {
		data, err := os.ReadFile("testdata/" + name)
		if err != nil {
			t.Fatal(err)
		}
		v, err := Parse(data)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		d := Dict(v)
		want := map[string]any{
			"Name":     "Mehran's iPhone",
			"Unicode":  "Fotos – été",
			"Count":    int64(42),
			"Big":      int64(1 << 40),
			"Negative": int64(-7),
			"Ratio":    0.5,
			"Yes":      true,
			"No":       false,
			"List":     []any{"a", int64(1), []any{int64(2), int64(3)}},
		}
		for k, w := range want {
			if !reflect.DeepEqual(d[k], w) {
				t.Errorf("%s: %s = %#v, want %#v", name, k, d[k], w)
			}
		}
		if when, _ := d["When"].(time.Time); !when.Equal(time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)) {
			t.Errorf("%s: When = %v", name, d["When"])
		}
		if blob, _ := d["Blob"].([]byte); len(blob) != 40 || blob[39] != 39 {
			t.Errorf("%s: Blob = %v", name, d["Blob"])
		}
		if String(d["Long"]) != string(bytes.Repeat([]byte("x"), 300)) {
			t.Errorf("%s: Long is %d bytes", name, len(String(d["Long"])))
		}
		if name == "binary.plist" && d["Ref"] != UID(5) {
			t.Errorf("Ref = %#v", d["Ref"])
		}
	}
}

func TestDamaged(t *testing.T) {
	data, _ := os.ReadFile("testdata/binary.plist")
	for i := 8; i < len(data); i += 7 {
		bad := bytes.Clone(data)
		bad[i] ^= 0xff
		Parse(bad) // must not panic
	}
}
