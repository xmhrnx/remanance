package usbmass

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"
)

// SCSI operation codes used to read a drive.
const (
	opTestUnitReady  = 0x00
	opRequestSense   = 0x03
	opInquiry        = 0x12
	opReadCapacity10 = 0x25
	opRead10         = 0x28
	opServiceIn16    = 0x9e // READ CAPACITY (16) is its action 0x10
	opRead16         = 0x88
)

// Sense describes why a command failed.
type Sense struct {
	Key, ASC, ASCQ byte
}

// SenseError is a failed command with its sense data.
type SenseError struct{ Sense Sense }

// scsi issues commands to one logical unit.
type scsi struct{ bot bot }

func (s Sense) String() string {
	switch {
	case s.Key == 0x02 && s.ASC == 0x3a:
		return "no medium in the drive"
	case s.Key == 0x02:
		return "the drive isn't ready"
	case s.Key == 0x03 || s.Key == 0x04:
		return "unreadable sector"
	case s.Key == 0x05:
		return "the drive doesn't support the command"
	case s.Key == 0x06:
		return "the medium changed"
	}
	return fmt.Sprintf("sense %02x/%02x/%02x", s.Key, s.ASC, s.ASCQ)
}

func (e *SenseError) Error() string { return "usb: " + e.Sense.String() }

// Medium reports a read error from the disk itself: a bad sector, worth
// skipping, unlike a lost connection.
func (e *SenseError) Medium() bool { return e.Sense.Key == 0x03 || e.Sense.Key == 0x04 }

func (s *scsi) run(cdb []byte, data []byte) (int, error) {
	n, err := s.bot.command(cdb, data, true)
	if errors.Is(err, errCheckCondition) {
		sense, serr := s.requestSense()
		if serr != nil {
			return n, err
		}
		return n, &SenseError{sense}
	}
	return n, err
}

func (s *scsi) requestSense() (Sense, error) {
	buf := make([]byte, 18)
	if _, err := s.bot.command([]byte{opRequestSense, 0, 0, 0, byte(len(buf)), 0}, buf, true); err != nil {
		return Sense{}, err
	}
	return Sense{Key: buf[2] & 0x0f, ASC: buf[12], ASCQ: buf[13]}, nil
}

// inquiry names the drive.
func (s *scsi) inquiry() (vendor, product string, removable bool, err error) {
	buf := make([]byte, 36)
	n, err := s.run([]byte{opInquiry, 0, 0, 0, byte(len(buf)), 0}, buf)
	if err != nil {
		return "", "", false, err
	}
	if n < 32 {
		return "", "", false, errors.New("usb: short inquiry data")
	}
	trim := func(b []byte) string { return strings.TrimSpace(strings.Trim(string(b), "\x00")) }
	return trim(buf[8:16]), trim(buf[16:32]), buf[1]&0x80 != 0, nil
}

// waitReady polls TEST UNIT READY: card readers and spinning drives take
// a moment after being plugged in.
func (s *scsi) waitReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		_, err := s.run([]byte{opTestUnitReady, 0, 0, 0, 0, 0}, nil)
		var se *SenseError
		if err == nil || !errors.As(err, &se) || se.Sense.Key != 0x02 && se.Sense.Key != 0x06 || time.Now().After(deadline) {
			return err
		}
		if se.Sense.Key == 0x02 && se.Sense.ASC == 0x3a {
			return err // no medium: waiting won't help
		}
		time.Sleep(250 * time.Millisecond)
	}
}

// capacity returns the number of blocks and the block size, through
// READ CAPACITY (16) for drives over 2 TiB.
func (s *scsi) capacity() (blocks uint64, blockSize uint32, err error) {
	buf := make([]byte, 8)
	if _, err = s.run([]byte{opReadCapacity10, 0, 0, 0, 0, 0, 0, 0, 0, 0}, buf); err != nil {
		return 0, 0, err
	}
	be := binary.BigEndian
	last, size := be.Uint32(buf[0:]), be.Uint32(buf[4:])
	if last != 0xffffffff {
		return uint64(last) + 1, size, nil
	}
	buf = make([]byte, 32)
	cdb := make([]byte, 16)
	cdb[0], cdb[1] = opServiceIn16, 0x10
	be.PutUint32(cdb[10:], uint32(len(buf)))
	if _, err = s.run(cdb, buf); err != nil {
		return 0, 0, err
	}
	return be.Uint64(buf[0:]) + 1, be.Uint32(buf[8:]), nil
}

// read reads count blocks from lba into buf.
func (s *scsi) read(lba uint64, count uint32, buf []byte, wide bool) (int, error) {
	be := binary.BigEndian
	var cdb []byte
	if wide {
		cdb = make([]byte, 16)
		cdb[0] = opRead16
		be.PutUint64(cdb[2:], lba)
		be.PutUint32(cdb[10:], count)
	} else {
		cdb = make([]byte, 10)
		cdb[0] = opRead10
		be.PutUint32(cdb[2:], uint32(lba))
		be.PutUint16(cdb[7:], uint16(count))
	}
	return s.run(cdb, buf)
}
