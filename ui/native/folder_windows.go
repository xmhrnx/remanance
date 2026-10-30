package native

import (
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	shell32             = windows.NewLazySystemDLL("shell32.dll")
	ole32               = windows.NewLazySystemDLL("ole32.dll")
	procBrowseForFolder = shell32.NewProc("SHBrowseForFolderW")
	procPathFromIDList  = shell32.NewProc("SHGetPathFromIDListW")
	procCoInitializeEx  = ole32.NewProc("CoInitializeEx")
	procCoUninitialize  = ole32.NewProc("CoUninitialize")
	procCoTaskMemFree   = ole32.NewProc("CoTaskMemFree")
)

type browseInfo struct {
	owner       uintptr
	root        uintptr
	displayName *uint16
	title       *uint16
	flags       uint32
	callback    uintptr
	param       uintptr
	image       int32
}

const (
	bifReturnOnlyFSDirs = 0x0001
	bifNewDialogStyle   = 0x0040
	coinitApartment     = 0x2
)

func pickFolder(title string) (string, error) {
	// The dialog needs COM in a single-threaded apartment on this thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	procCoInitializeEx.Call(0, coinitApartment)
	defer procCoUninitialize.Call()

	name := make([]uint16, windows.MAX_PATH)
	t, _ := syscall.UTF16PtrFromString(title)
	bi := browseInfo{displayName: &name[0], title: t, flags: bifReturnOnlyFSDirs | bifNewDialogStyle}
	pidl, _, _ := procBrowseForFolder.Call(uintptr(unsafe.Pointer(&bi)))
	if pidl == 0 {
		return "", ErrCanceled
	}
	defer procCoTaskMemFree.Call(pidl)
	buf := make([]uint16, 32768)
	if ok, _, _ := procPathFromIDList.Call(pidl, uintptr(unsafe.Pointer(&buf[0]))); ok == 0 {
		return "", ErrUnsupported // a virtual folder, such as This PC
	}
	return windows.UTF16ToString(buf), nil
}
