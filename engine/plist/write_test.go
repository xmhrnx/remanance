package plist

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestMarshal(t *testing.T) {
	when := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	v := map[string]any{
		"Name":   "Sam's iPhone <3> & été",
		"Count":  int64(42),
		"Big":    int64(1 << 40),
		"Ratio":  0.25,
		"Yes":    true,
		"Data":   []byte{0, 1, 2, 250},
		"When":   when,
		"List":   []any{"DLMessageVersionExchange", "DLVersionsOk", int64(300)},
		"Nested": map[string]any{"Long": strings.Repeat("x", 300)},
	}
	for name, marshal := range map[string]func(any) ([]byte, error){"xml": MarshalXML, "binary": MarshalBinary} {
		data, err := marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		got, err := Parse(data)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		d := Dict(got)
		if w, _ := d["When"].(time.Time); !w.Equal(when) {
			t.Errorf("%s: When = %v", name, d["When"])
		}
		delete(d, "When")
		want := map[string]any{}
		for k, x := range v {
			if k != "When" {
				want[k] = x
			}
		}
		if !reflect.DeepEqual(d, want) {
			t.Errorf("%s: %#v", name, d)
		}
	}
}
