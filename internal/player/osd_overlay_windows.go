//go:build windows

package player

import (
	"fmt"

	"github.com/gen2brain/go-mpv"
)

// osdOverlaySet sends an osd-overlay command via mpv_command_string.
// On Windows, CGO is unavailable so we can't use mpv_command_node.
// Instead, we use positional arguments which mpv parses the same way.
func osdOverlaySet(m *mpv.Mpv, id int, data string, resX, resY int) error {
	return m.CommandString(mpvCmd("osd-overlay",
		fmt.Sprintf("%d", id),
		"ass-events",
		data,
		fmt.Sprintf("%d", resX),
		fmt.Sprintf("%d", resY),
	))
}

// osdOverlayRemove removes an osd-overlay by setting format to "none".
// An empty data arg is required because mpv's positional parser treats it as mandatory.
func osdOverlayRemove(m *mpv.Mpv, id int) error {
	return m.CommandString(mpvCmd("osd-overlay",
		fmt.Sprintf("%d", id),
		"none",
		"",
	))
}
