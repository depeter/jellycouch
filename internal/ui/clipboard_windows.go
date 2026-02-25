//go:build windows

package ui

import (
	"syscall"
	"unsafe"
)

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procOpenCB       = user32.NewProc("OpenClipboard")
	procCloseCB      = user32.NewProc("CloseClipboard")
	procEmptyCB      = user32.NewProc("EmptyClipboard")
	procGetCBData    = user32.NewProc("GetClipboardData")
	procSetCBData    = user32.NewProc("SetClipboardData")
	procGlobalAlloc  = kernel32.NewProc("GlobalAlloc")
	procGlobalLock   = kernel32.NewProc("GlobalLock")
	procGlobalUnlock = kernel32.NewProc("GlobalUnlock")
)

const (
	cfUnicodeText = 13
	gmemMoveable  = 0x0002
)

func readClipboard() string {
	ret, _, _ := procOpenCB.Call(0)
	if ret == 0 {
		return ""
	}
	defer procCloseCB.Call()

	h, _, _ := procGetCBData.Call(cfUnicodeText)
	if h == 0 {
		return ""
	}

	ptr, _, _ := procGlobalLock.Call(h)
	if ptr == 0 {
		return ""
	}
	defer procGlobalUnlock.Call(h)

	return syscall.UTF16ToString((*[1 << 20]uint16)(unsafe.Pointer(ptr))[:])
}

func writeClipboard(text string) {
	ret, _, _ := procOpenCB.Call(0)
	if ret == 0 {
		return
	}
	defer procCloseCB.Call()

	procEmptyCB.Call()

	utf16, _ := syscall.UTF16FromString(text)
	size := len(utf16) * 2

	h, _, _ := procGlobalAlloc.Call(gmemMoveable, uintptr(size))
	if h == 0 {
		return
	}

	ptr, _, _ := procGlobalLock.Call(h)
	if ptr == 0 {
		return
	}

	src := unsafe.Pointer(&utf16[0])
	copy((*[1 << 20]byte)(unsafe.Pointer(ptr))[:size], (*[1 << 20]byte)(src)[:size])

	procGlobalUnlock.Call(h)
	procSetCBData.Call(cfUnicodeText, h)
}
