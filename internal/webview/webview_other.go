//go:build !linux

package webview

import (
	"errors"
	"os/exec"
)

// RunWebview is not supported on this platform.
func RunWebview(url string) {
	panic("webview: not supported on this platform")
}

// StartWebApp is not supported on this platform.
func StartWebApp(url string) (*exec.Cmd, error) {
	return nil, errors.New("webview: not supported on this platform")
}
