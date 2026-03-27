//go:build windows

package player

import "github.com/gen2brain/go-mpv"

// osdOverlaySet is a no-op stub on Windows (mpv_command_node requires cgo).
func osdOverlaySet(m *mpv.Mpv, id int, data string, resX, resY int) error {
	return nil
}

// osdOverlayRemove is a no-op stub on Windows.
func osdOverlayRemove(m *mpv.Mpv, id int) error {
	return nil
}
