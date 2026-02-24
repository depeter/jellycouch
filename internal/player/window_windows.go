//go:build windows

package player

import (
	"fmt"
	"syscall"
)

var (
	user32                = syscall.NewLazyDLL("user32.dll")
	procGetForegroundWnd  = user32.NewProc("GetForegroundWindow")
)

// GetWindowHandle returns the Win32 HWND of the foreground window.
func GetWindowHandle() (int64, error) {
	hwnd, _, _ := procGetForegroundWnd.Call()
	if hwnd == 0 {
		return 0, fmt.Errorf("failed to get foreground window handle")
	}
	return int64(hwnd), nil
}
