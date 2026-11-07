package plist

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"slices"
	"time"
)

// MarshalXML writes v as an XML property list. v is made of the types
// Parse returns, plus int, uint64 and []string.
func MarshalXML(v any) ([]byte, error) {
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" +
		`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n" +
		`<plist version="1.0">` + "\n")
	if err := writeXML(&b, v); err != nil {
		return nil, err
	}
	b.WriteString("\n</plist>\n")
	return b.Bytes(), nil
}

func writeXML(b *bytes.Buffer, v any) error {
	switch v := v.(type) {
	case map[string]any:
		b.WriteString("<dict>")
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		for _, k := range keys {
			b.WriteString("<key>")
			xml.EscapeText(b, []byte(k))
			b.WriteString("</key>")
			if err := writeXML(b, v[k]); err != nil {
				return err
			}
		}
		b.WriteString("</dict>")
	case []any:
		b.WriteString("<array>")
		for _, x := range v {
			if err := writeXML(b, x); err != nil {
				return err
			}
		}
		b.WriteString("</array>")
	case []string:
		b.WriteString("<array>")
		for _, x := range v {
			writeXML(b, x)
		}
		b.WriteString("</array>")
	case string:
		b.WriteString("<string>")
		xml.EscapeText(b, []byte(v))
		b.WriteString("</string>")
	case int, int64, uint64, UID:
		fmt.Fprintf(b, "<integer>%d</integer>", v)
	case float64:
		fmt.Fprintf(b, "<real>%g</real>", v)
	case bool:
		if v {
			b.WriteString("<true/>")
		} else {
			b.WriteString("<false/>")
		}
	case []byte:
		fmt.Fprintf(b, "<data>%s</data>", base64.StdEncoding.EncodeToString(v))
	case time.Time:
		fmt.Fprintf(b, "<date>%s</date>", v.UTC().Format(time.RFC3339))
	default:
		return fmt.Errorf("plist: can't write a %T", v)
	}
	return nil
}
