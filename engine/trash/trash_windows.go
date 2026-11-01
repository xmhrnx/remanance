package trash

import (
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const supported = true

var shFileOperation = windows.NewLazySystemDLL("shell32.dll").NewProc("SHFileOperationW")

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
