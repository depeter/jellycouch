//go:build linux

package player

/*
#cgo LDFLAGS: -lX11
#include <X11/Xlib.h>

long getActiveWindowX11() {
    Display *d = XOpenDisplay(NULL);
    if (!d) return 0;
    Window w;
    int revert;
    XGetInputFocus(d, &w, &revert);
    XCloseDisplay(d);
    return (long)w;
}
*/
import "C"

import "fmt"

// GetWindowHandle returns the X11 window ID of the currently focused window.
func GetWindowHandle() (int64, error) {
	wid := int64(C.getActiveWindowX11())
	if wid == 0 {
		return 0, fmt.Errorf("failed to get X11 window handle")
	}
	return wid, nil
}
