package plist

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"slices"
	"time"
	"unicode/utf16"
)

// binaryObject is one object of a binary property list.
type binaryObject struct {
	head []byte // the marker and the object's own bytes
	refs []int  // the objects a dict or array holds: keys, then values
}

// binaryWriter collects the objects a binary property list holds.
type binaryWriter struct {
	objects []binaryObject
}

// MarshalBinary writes v as a binary property list, of the same types
// as MarshalXML.
func MarshalBinary(v any) ([]byte, error) {
	w := &binaryWriter{}
	if _, err := w.add(v); err != nil {
		return nil, err
	}
	n := len(w.objects)
	refSize := sizeFor(uint64(n))
	// Flatten: each object's bytes, references written with refSize.
	var body bytes.Buffer
	body.WriteString("bplist00")
	offsets := make([]uint64, n)
	for i, o := range w.objects {
		offsets[i] = uint64(body.Len())
		body.Write(o.head)
		for _, r := range o.refs {
			putN(&body, uint64(r), refSize)
		}
	}
	table := uint64(body.Len())
	offSize := sizeFor(table)
	for _, o := range offsets {
		putN(&body, o, offSize)
	}
	trailer := make([]byte, 32)
	trailer[6] = byte(offSize)
	trailer[7] = byte(refSize)
	binary.BigEndian.PutUint64(trailer[8:], uint64(n))
	binary.BigEndian.PutUint64(trailer[16:], 0)
	binary.BigEndian.PutUint64(trailer[24:], table)
	body.Write(trailer)
	return body.Bytes(), nil
}

// add appends v and what it holds, and returns its index.
func (w *binaryWriter) add(v any) (int, error) {
	i := len(w.objects)
	w.objects = append(w.objects, binaryObject{})
	var o binaryObject
	switch v := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		o.head = marker(0xd, len(keys))
		for _, k := range keys {
			r, _ := w.add(k)
			o.refs = append(o.refs, r)
		}
		for _, k := range keys {
			r, err := w.add(v[k])
			if err != nil {
				return 0, err
			}
			o.refs = append(o.refs, r)
		}
	case []any:
		o.head = marker(0xa, len(v))
		for _, x := range v {
			r, err := w.add(x)
			if err != nil {
				return 0, err
			}
			o.refs = append(o.refs, r)
		}
	case []string:
		o.head = marker(0xa, len(v))
		for _, x := range v {
			r, _ := w.add(x)
			o.refs = append(o.refs, r)
		}
	case string:
		ascii := true
		for i := 0; i < len(v); i++ {
			if v[i] >= 0x80 {
				ascii = false
				break
			}
		}
		if ascii {
			o.head = append(marker(0x5, len(v)), v...)
		} else {
			u := utf16.Encode([]rune(v))
			o.head = marker(0x6, len(u))
			for _, c := range u {
				o.head = binary.BigEndian.AppendUint16(o.head, c)
			}
		}
	case int:
		o.head = intObject(int64(v))
	case int64:
		o.head = intObject(v)
	case uint64:
		o.head = intObject(int64(v))
	case UID:
		o.head = append([]byte{0x80 | 7}, binary.BigEndian.AppendUint64(nil, uint64(v))...)
	case float64:
		o.head = append([]byte{0x23}, binary.BigEndian.AppendUint64(nil, math.Float64bits(v))...)
	case bool:
		o.head = []byte{0x08}
		if v {
			o.head = []byte{0x09}
		}
	case []byte:
		o.head = append(marker(0x4, len(v)), v...)
	case time.Time:
		s := v.Sub(time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)).Seconds()
		o.head = append([]byte{0x33}, binary.BigEndian.AppendUint64(nil, math.Float64bits(s))...)
	default:
		return 0, fmt.Errorf("plist: can't write a %T", v)
	}
	w.objects[i] = o
	return i, nil
}
