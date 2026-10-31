package iphone

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xmhrnx/remanance/engine/plist"
)

// The backup service speaks DeviceLink: the phone sends requests as
// arrays, the name first ("DLMessageUploadFiles"), and the computer
// answers each with a status. Files go either way as raw bytes on the
// same connection: each file's name after its length, then blocks of
// its data, each after its length and a code.
const (
	codeSuccess   = 0x00
	codeError     = 0x06
	codeFileData  = 0x0c
	emptyParam    = "___EmptyParameterString___"
	maxNameLength = 4096
)

// link answers the phone's DeviceLink requests in a backup folder.
type link struct {
	c    *conn
	root string // where the phone's paths start: the folder of backups
}

// status answers a request: 0 for done, with a value if it asked for
// one.
func (l *link) status(code int64, message string, value any) error {
	if message == "" {
		message = emptyParam
	}
	if value == nil {
		value = map[string]any{}
	}
	return l.c.sendBinary([]any{"DLMessageStatusResponse", code, message, value})
}

// path turns a path the phone gives into one in the backup folder,
// refusing any that would leave it.
func (l *link) path(p string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(p))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("the iPhone asked for %q, outside the backup", p)
	}
	return filepath.Join(l.root, clean), nil
}

func (l *link) writeUint32(n uint32) error {
	_, err := l.c.Write(binary.BigEndian.AppendUint32(nil, n))
	return err
}

func (l *link) readUint32() (uint32, error) {
	b := make([]byte, 4)
	_, err := io.ReadFull(l.c, b)
	return binary.BigEndian.Uint32(b), err
}

// sendFiles sends the phone the files it asks for, from an earlier
// backup, which tells it what changed since.
func (l *link) sendFiles(paths []any) error {
	failed := map[string]any{}
	for _, x := range paths {
		p := plist.String(x)
		if err := l.writeUint32(uint32(len(p))); err != nil {
			return err
		}
		if _, err := l.c.Write([]byte(p)); err != nil {
			return err
		}
		if err := l.sendFile(p); err != nil {
			if !errors.Is(err, errSent) {
				return err
			}
			failed[p] = map[string]any{"DLFileErrorString": "No such file or directory", "DLFileErrorCode": int64(-6)}
		}
	}
	if err := l.writeUint32(0); err != nil {
		return err
	}
	if len(failed) > 0 {
		return l.status(-13, "Multi status", failed)
	}
	return l.status(0, "", nil)
}

// errSent marks a file the phone was told is missing.
var errSent = errors.New("missing file, sent as an error")

func (l *link) sendFile(p string) error {
	full, err := l.path(p)
	var f *os.File
	if err == nil {
		f, err = os.Open(full)
	}
	if err != nil {
		msg := "No such file or directory"
		if err := l.writeUint32(uint32(len(msg) + 1)); err != nil {
			return err
		}
		if _, err := l.c.Write(append([]byte{codeError}, msg...)); err != nil {
			return err
		}
		return errSent
	}
	defer f.Close()
	buf := make([]byte, 1+64<<10)
	for {
		buf[0] = codeFileData
		n, rerr := f.Read(buf[1:])
		if n > 0 {
			if err := l.writeUint32(uint32(n + 1)); err != nil {
				return err
			}
			if _, err := l.c.Write(buf[:n+1]); err != nil {
				return err
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}
	if err := l.writeUint32(1); err != nil {
		return err
	}
	_, err = l.c.Write([]byte{codeSuccess})
	return err
}

// receiveFiles writes the files the phone sends into the backup
// folder, until it sends an empty name.
func (l *link) receiveFiles(onFile func(n int64)) error {
	var failed []string
	for {
		if _, ok, err := l.readName(); err != nil || !ok {
			if err != nil {
				return err
			}
			break // the device's name of the file, then the backup's
		}
		name, ok, err := l.readName()
		if err != nil {
			return err
		}
		if !ok {
			break
		}
		full, err := l.path(name)
		if err != nil {
			return err
		}
		os.MkdirAll(filepath.Dir(full), 0o755)
		os.Remove(full)
		f, ferr := os.Create(full)
		var written int64
		for {
			n, err := l.readUint32()
			if err != nil {
				closeFile(f)
				return err
			}
			code := make([]byte, 1)
			if _, err := io.ReadFull(l.c, code); err != nil {
				closeFile(f)
				return err
			}
			if n == 0 {
				break
			}
			data := make([]byte, n-1)
			if _, err := io.ReadFull(l.c, data); err != nil {
				closeFile(f)
				return err
			}
			if code[0] == codeFileData {
				if ferr == nil {
					if _, err := f.Write(data); err != nil {
						ferr = err
					}
				}
				written += int64(len(data))
				continue
			}
			if code[0] == codeError {
				failed = append(failed, name+": "+string(data))
			}
			break
		}
		closeFile(f)
		if ferr != nil {
			return fmt.Errorf("saving the backup: %w", ferr)
		}
		if onFile != nil {
			onFile(written)
		}
	}
	return l.status(0, "", nil)
}

func closeFile(f *os.File) {
	if f != nil {
		f.Close()
	}
}

// readName reads a file's name after its length; ok is false for the
// empty name that ends the files.
func (l *link) readName() (string, bool, error) {
	n, err := l.readUint32()
	if err != nil || n == 0 {
		return "", false, err
	}
	if n > maxNameLength {
		return "", false, fmt.Errorf("the iPhone sent a file name of %d bytes", n)
	}
	b := make([]byte, n)
	if _, err := io.ReadFull(l.c, b); err != nil {
		return "", false, err
	}
	return string(b), true, nil
}

// contents lists a folder of the backup for the phone.
func (l *link) contents(p string) error {
	full, err := l.path(p)
	if err != nil {
		return err
	}
	out := map[string]any{}
	entries, _ := os.ReadDir(full)
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		kind := "DLFileTypeRegular"
		if e.IsDir() {
			kind = "DLFileTypeDirectory"
		}
		out[e.Name()] = map[string]any{"DLFileType": kind, "DLFileSize": info.Size(), "DLFileModificationDate": info.ModTime()}
	}
	return l.status(0, "", out)
}

func (l *link) mkdir(p string) error {
	full, err := l.path(p)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(full, 0o755); err != nil {
		return l.status(-1, err.Error(), nil)
	}
	return l.status(0, "", nil)
}

// move moves files within the backup: a map of where from, to where.
func (l *link) move(moves map[string]any) error {
	for from, to := range moves {
		src, err := l.path(from)
		if err != nil {
			return err
		}
		dst, err := l.path(plist.String(to))
		if err != nil {
			return err
		}
		os.RemoveAll(dst)
		os.MkdirAll(filepath.Dir(dst), 0o755)
		if err := os.Rename(src, dst); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return l.status(-1, err.Error(), nil)
		}
	}
	return l.status(0, "", nil)
}

func (l *link) remove(paths []any) error {
	for _, x := range paths {
		full, err := l.path(plist.String(x))
		if err != nil {
			return err
		}
		os.RemoveAll(full)
	}
	return l.status(0, "", nil)
}

// copyItem copies a file or folder within the backup.
func (l *link) copyItem(from, to string) error {
	src, err := l.path(from)
	if err != nil {
		return err
	}
	dst, err := l.path(to)
	if err != nil {
		return err
	}
	err = filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return l.status(-1, err.Error(), nil)
	}
	return l.status(0, "", nil)
}

// progressOf finds the overall progress, 0 to 100, a request carries.
func progressOf(msg []any) (float64, bool) {
	for _, i := range []int{3, 2} {
		if i < len(msg) {
			if f, ok := msg[i].(float64); ok && f > 0 && f <= 100 {
				return f, true
			}
		}
	}
	return 0, false
}

// infoPlist is the Info.plist the computer writes into a backup, which
// says whose it is.
func infoPlist(values map[string]any) ([]byte, error) {
	info := map[string]any{"Last Backup Date": time.Now().UTC()}
	for k, v := range values {
		if v != nil {
			info[k] = v
		}
	}
	return plist.MarshalXML(info)
}
