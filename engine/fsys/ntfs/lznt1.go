package ntfs

import (
	"encoding/binary"
	"errors"
	"io"

	"github.com/mehran9675/remanence/engine/fsys"
)

const lznt1Chunk = 4096

var errLZNT1 = errors.New("ntfs: corrupt compressed data")

// decompressLZNT1 decodes NTFS's LZ77 variant into out, which has room
// for the whole compression unit, and returns how many bytes it wrote.
func decompressLZNT1(in, out []byte) (int, error) {
	o := 0
	for len(in) >= 2 && o < len(out) {
		h := binary.LittleEndian.Uint16(in)
		if h == 0 {
			break // end of compressed data; the rest of the unit is zeros
		}
		size := int(h&0x0FFF) + 3
		if size > len(in) {
			size = len(in)
		}
		chunk := in[2:size]
		in = in[size:]
		end := min(o+lznt1Chunk, len(out))
		if h&0x8000 == 0 {
			o += copy(out[o:end], chunk)
			continue
		}
		start := o
		for i := 0; i < len(chunk) && o < end; {
			flags := chunk[i]
			i++
			for bit := 0; bit < 8 && i < len(chunk) && o < end; bit++ {
				if flags&(1<<bit) == 0 {
					out[o] = chunk[i]
					o++
					i++
					continue
				}
				if i+2 > len(chunk) {
					return o, errLZNT1
				}
				tok := int(binary.LittleEndian.Uint16(chunk[i:]))
				i += 2
				lenBits := 12
				for p := o - start - 1; p >= 0x10; p >>= 1 {
					lenBits--
				}
				length := tok&(1<<lenBits-1) + 3
				back := tok>>lenBits + 1
				if back > o-start {
					return o, errLZNT1
				}
				for k := 0; k < length && o < end; k++ {
					out[o] = out[o-back]
					o++
				}
			}
		}
		// A compressed chunk shorter than 4 KB is padded with zeros up to
		// the next chunk.
		if o < end && len(in) >= 2 && binary.LittleEndian.Uint16(in) != 0 {
			for o < end {
				out[o] = 0
				o++
			}
		}
	}
	return o, nil
}

// readUnit is the fsys codec for compressed NTFS files: it returns the
// plain bytes of compression unit n.
func readUnit(dev io.ReaderAt, e *fsys.Entry, n int64) ([]byte, error) {
	unit := e.Unit
	start, end := n*unit, (n+1)*unit
	var stored []byte
	sparse := true
	pos := int64(0)
	for _, r := range e.Runs {
		rEnd := pos + r.Length
		if rEnd > start && pos < end {
			from := max(start, pos)
			to := min(end, rEnd)
			if r.Offset >= 0 {
				sparse = false
				b := make([]byte, to-from)
				if _, err := dev.ReadAt(b, r.Offset+(from-pos)); err != nil && err != io.EOF {
					return nil, err
				}
				stored = append(stored, b...)
			}
		}
		pos = rEnd
		if pos >= end {
			break
		}
	}
	out := make([]byte, unit)
	if sparse {
		return out, nil
	}
	if int64(len(stored)) >= unit {
		copy(out, stored) // the unit didn't compress and is stored as is
		return out, nil
	}
	if _, err := decompressLZNT1(stored, out); err != nil {
		return out, err
	}
	return out, nil
}

func init() { fsys.RegisterCodec("lznt1", readUnit) }
