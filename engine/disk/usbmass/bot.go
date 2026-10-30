package usbmass

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

// Bulk-Only Transport framing: every command is a 31-byte Command Block
// Wrapper, an optional data phase, and a 13-byte Command Status Wrapper.
const (
	cbwSignature = 0x43425355 // "USBC"
	cswSignature = 0x53425355 // "USBS"
	cbwLen       = 31
	cswLen       = 13

	statusPassed     = 0
	statusFailed     = 1
	statusPhaseError = 2
)

// commandTimeout bounds one command; a failing drive can take long to
// give up on a sector, so it's generous.
const commandTimeout = 20 * time.Second

// errCheckCondition is a command the drive ran and failed; REQUEST SENSE
// says why.
var errCheckCondition = errors.New("usb: command failed")

type bot struct {
	t   Transport
	tag uint32
	lun uint8
}

func (b *bot) cbw(cdb []byte, dataLen int, in bool) []byte {
	b.tag++
	w := make([]byte, cbwLen)
	le := binary.LittleEndian
	le.PutUint32(w[0:], cbwSignature)
	le.PutUint32(w[4:], b.tag)
	le.PutUint32(w[8:], uint32(dataLen))
	if in {
		w[12] = 0x80
	}
	w[13] = b.lun
	w[14] = byte(len(cdb))
	copy(w[15:], cdb)
	return w
}

// command runs one SCSI command. data is read into (in) or written from;
// it returns how many bytes moved in the data phase.
func (b *bot) command(cdb []byte, data []byte, in bool) (int, error) {
	if _, err := b.t.BulkOut(b.cbw(cdb, len(data), in), commandTimeout); err != nil {
		b.t.Reset()
		return 0, fmt.Errorf("usb: sending command: %w", err)
	}
	moved, dataErr := b.dataPhase(data, in)
	csw, err := b.status()
	if err != nil {
		b.t.Reset()
		return moved, err
	}
	le := binary.LittleEndian
	if le.Uint32(csw[0:]) != cswSignature || le.Uint32(csw[4:]) != b.tag {
		b.t.Reset()
		return moved, errors.New("usb: the drive answered out of step; it was reset")
	}
	residue := int(le.Uint32(csw[8:]))
	switch csw[12] {
	case statusPassed:
		if dataErr != nil {
			return moved, dataErr
		}
		if in && residue > 0 && len(data)-residue < moved {
			moved = len(data) - residue
		}
		return moved, nil
	case statusFailed:
		return moved, errCheckCondition
	default:
		b.t.Reset()
		return moved, errors.New("usb: phase error; the drive was reset")
	}
}

// dataPhase moves the data in transfers the transport can take. A stall
// ends it early: the drive has no more to give, and the status says why.
func (b *bot) dataPhase(data []byte, in bool) (int, error) {
	moved := 0
	max := b.t.MaxTransfer()
	for moved < len(data) {
		chunk := data[moved:min(len(data), moved+max)]
		var n int
		var err error
		if in {
			n, err = b.t.BulkIn(chunk, commandTimeout)
		} else {
			n, err = b.t.BulkOut(chunk, commandTimeout)
		}
		moved += n
		if errors.Is(err, ErrStall) {
			b.t.ClearHalt(in)
			return moved, nil
		}
		if err != nil {
			return moved, err
		}
		if in && n < len(chunk) {
			return moved, nil // a short packet ends the data phase
		}
	}
	return moved, nil
}

// status reads the CSW, clearing one stall on the way as the
// specification allows.
func (b *bot) status() ([]byte, error) {
	csw := make([]byte, cswLen)
	for attempt := 0; attempt < 2; attempt++ {
		n, err := b.t.BulkIn(csw, commandTimeout)
		if errors.Is(err, ErrStall) {
			b.t.ClearHalt(true)
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("usb: reading status: %w", err)
		}
		if n != cswLen {
			return nil, fmt.Errorf("usb: status of %d bytes", n)
		}
		return csw, nil
	}
	return nil, errors.New("usb: the drive keeps refusing to report status")
}
