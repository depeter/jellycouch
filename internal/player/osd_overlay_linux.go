//go:build linux

package player

/*
#include <mpv/client.h>
#include <stdlib.h>
#include <string.h>

// osd_overlay_set calls mpv_command_node with an osd-overlay map.
static int osd_overlay_set(mpv_handle *h, int id, const char *data, int res_x, int res_y) {
    mpv_node keys_vals[6];
    char *keys[6];

    // "name" -> "osd-overlay"
    keys[0] = "name";
    keys_vals[0].format = MPV_FORMAT_STRING;
    keys_vals[0].u.string = "osd-overlay";

    // "id" -> id
    keys[1] = "id";
    keys_vals[1].format = MPV_FORMAT_INT64;
    keys_vals[1].u.int64 = id;

    // "format" -> "ass" (full ASS script for reliable alignment control)
    keys[2] = "format";
    keys_vals[2].format = MPV_FORMAT_STRING;
    keys_vals[2].u.string = "ass";

    // "data" -> data
    keys[3] = "data";
    keys_vals[3].format = MPV_FORMAT_STRING;
    keys_vals[3].u.string = (char*)data;

    // "res_x" -> res_x
    keys[4] = "res_x";
    keys_vals[4].format = MPV_FORMAT_INT64;
    keys_vals[4].u.int64 = res_x;

    // "res_y" -> res_y
    keys[5] = "res_y";
    keys_vals[5].format = MPV_FORMAT_INT64;
    keys_vals[5].u.int64 = res_y;

    mpv_node_list list = {
        .num    = 6,
        .values = keys_vals,
        .keys   = keys,
    };

    mpv_node cmd;
    cmd.format = MPV_FORMAT_NODE_MAP;
    cmd.u.list = &list;

    mpv_node result;
    int err = mpv_command_node(h, &cmd, &result);
    if (err >= 0) {
        mpv_free_node_contents(&result);
    }
    return err;
}

// osd_overlay_remove removes an osd-overlay by setting format to "none".
static int osd_overlay_remove(mpv_handle *h, int id) {
    mpv_node keys_vals[3];
    char *keys[3];

    keys[0] = "name";
    keys_vals[0].format = MPV_FORMAT_STRING;
    keys_vals[0].u.string = "osd-overlay";

    keys[1] = "id";
    keys_vals[1].format = MPV_FORMAT_INT64;
    keys_vals[1].u.int64 = id;

    keys[2] = "format";
    keys_vals[2].format = MPV_FORMAT_STRING;
    keys_vals[2].u.string = "none";

    mpv_node_list list = {
        .num    = 3,
        .values = keys_vals,
        .keys   = keys,
    };

    mpv_node cmd;
    cmd.format = MPV_FORMAT_NODE_MAP;
    cmd.u.list = &list;

    mpv_node result;
    int err = mpv_command_node(h, &cmd, &result);
    if (err >= 0) {
        mpv_free_node_contents(&result);
    }
    return err;
}
*/
import "C"

import (
	"fmt"
	"strings"
	"unsafe"

	"github.com/gen2brain/go-mpv"
)

// buildASSScript wraps raw ASS event lines in a complete ASS script.
// Each newline-separated line in data becomes a Dialogue event.
func buildASSScript(data string, resX, resY int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[Script Info]\nScriptType: v4.00+\nPlayResX: %d\nPlayResY: %d\n\n", resX, resY)
	b.WriteString("[V4+ Styles]\n")
	b.WriteString("Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n")
	b.WriteString("Style: Default,sans-serif,20,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,0,8,10,10,10,1\n\n")
	b.WriteString("[Events]\n")
	b.WriteString("Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")
	for _, line := range strings.Split(data, "\n") {
		if line != "" {
			fmt.Fprintf(&b, "Dialogue: 0,0:00:00.00,9:59:59.00,Default,,0,0,0,,%s\n", line)
		}
	}
	return b.String()
}

// osdOverlaySet sends an osd-overlay command to mpv via mpv_command_node.
// The mpv.Mpv struct's first (and only) field is *C.mpv_handle.
func osdOverlaySet(m *mpv.Mpv, id int, data string, resX, resY int) error {
	handle := *(**C.mpv_handle)(unsafe.Pointer(m))
	script := buildASSScript(data, resX, resY)
	cData := C.CString(script)
	defer C.free(unsafe.Pointer(cData))
	rc := C.osd_overlay_set(handle, C.int(id), cData, C.int(resX), C.int(resY))
	if rc < 0 {
		return fmt.Errorf("osd-overlay set: %d", rc)
	}
	return nil
}

// osdOverlayRemove removes an osd-overlay by ID.
func osdOverlayRemove(m *mpv.Mpv, id int) error {
	handle := *(**C.mpv_handle)(unsafe.Pointer(m))
	rc := C.osd_overlay_remove(handle, C.int(id))
	if rc < 0 {
		return fmt.Errorf("osd-overlay remove: %d", rc)
	}
	return nil
}
