package trash

import (
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const supported = true

var (
	shell32           = windows.NewLazySystemDLL("shell32.dll")
	shFileOperation   = shell32.NewProc("SHFileOperationW")
	shQueryRecycleBin = shell32.NewProc("SHQueryRecycleBinW")
	shEmptyRecycleBin = shell32.NewProc("SHEmptyRecycleBinW")
)

// shFileOp is SHFILEOPSTRUCTW.
type shFileOp struct {
	hwnd     uintptr
	op       uint32
	from     *uint16
	to       *uint16
	flags    uint16
	aborted  int32
	mappings uintptr
	title    *uint16
}

// shQueryInfo is SHQUERYRBINFO.
type shQueryInfo struct {
	size  uint32
	bytes int64
	items int64
}

const (
	foDelete          = 3
	fofSilent         = 0x0004
	fofNoConfirm      = 0x0010
	fofAllowUndo      = 0x0040
	fofNoErrorUI      = 0x0400
	fofNoConfirmMkdir = 0x0200
)

// move asks the shell to delete the file with undo allowed, which puts
// it in the Recycle Bin.
func move(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	// The list of paths ends with an empty one.
	from, err := syscall.UTF16FromString(abs)
	if err != nil {
		return err
	}
	from = append(from, 0)
	op := shFileOp{op: foDelete, from: &from[0], flags: fofSilent | fofNoConfirm | fofAllowUndo | fofNoErrorUI | fofNoConfirmMkdir}
	r, _, _ := shFileOperation.Call(uintptr(unsafe.Pointer(&op)))
	if r != 0 || op.aborted != 0 {
		return ErrNoTrash
	}
	return nil
}

// usage asks the shell what the Recycle Bins of all drives hold.
func usage() (Usage, error) {
	q := shQueryInfo{size: uint32(unsafe.Sizeof(shQueryInfo{}))}
	if r, _, _ := shQueryRecycleBin.Call(0, uintptr(unsafe.Pointer(&q))); r != 0 {
		return Usage{}, syscall.Errno(r)
	}
	return Usage{Bytes: q.bytes, Files: int(q.items)}, nil
}

const (
	sherbNoConfirmation = 0x1
	sherbNoProgressUI   = 0x2
	sherbNoSound        = 0x4
	eUnexpected         = 0x8000FFFF // what an empty bin answers
)

// empty empties the Recycle Bins of all drives, quietly.
func empty() error {
	r, _, _ := shEmptyRecycleBin.Call(0, 0, sherbNoConfirmation|sherbNoProgressUI|sherbNoSound)
	if r != 0 && r != eUnexpected {
		return syscall.Errno(r)
	}
	return nil
}
