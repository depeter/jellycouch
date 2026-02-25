// +build linux freebsd netbsd openbsd

package ui

/*
#cgo LDFLAGS: -lX11
#include <X11/Xlib.h>
#include <X11/Xatom.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

// readX11Clipboard reads the CLIPBOARD selection from the X server.
// Returns a malloc'd string (caller must free) or NULL on failure.
static char* readX11Clipboard() {
    Display *dpy = XOpenDisplay(NULL);
    if (!dpy) return NULL;

    Window win = XCreateSimpleWindow(dpy, DefaultRootWindow(dpy), 0, 0, 1, 1, 0, 0, 0);
    Atom clipboard = XInternAtom(dpy, "CLIPBOARD", False);
    Atom utf8str = XInternAtom(dpy, "UTF8_STRING", False);
    Atom target = XInternAtom(dpy, "JELLYCOUCH_CLIP", False);

    XConvertSelection(dpy, clipboard, utf8str, target, win, CurrentTime);
    XFlush(dpy);

    // Wait for SelectionNotify with timeout
    XEvent ev;
    struct timespec start, now;
    clock_gettime(CLOCK_MONOTONIC, &start);

    char *result = NULL;
    while (1) {
        clock_gettime(CLOCK_MONOTONIC, &now);
        double elapsed = (now.tv_sec - start.tv_sec) + (now.tv_nsec - start.tv_nsec) / 1e9;
        if (elapsed > 1.0) break;  // 1 second timeout

        if (XPending(dpy) > 0) {
            XNextEvent(dpy, &ev);
            if (ev.type == SelectionNotify) {
                if (ev.xselection.property != None) {
                    Atom actual_type;
                    int actual_format;
                    unsigned long nitems, bytes_after;
                    unsigned char *data = NULL;

                    XGetWindowProperty(dpy, win, target, 0, 1024*1024, True,
                        AnyPropertyType, &actual_type, &actual_format,
                        &nitems, &bytes_after, &data);

                    if (data && nitems > 0) {
                        result = (char*)malloc(nitems + 1);
                        memcpy(result, data, nitems);
                        result[nitems] = '\0';
                    }
                    if (data) XFree(data);
                }
                break;
            }
        } else {
            struct timespec ts = {0, 10000000}; // 10ms
            nanosleep(&ts, NULL);
        }
    }

    XDestroyWindow(dpy, win);
    XCloseDisplay(dpy);
    return result;
}
*/
import "C"
import "unsafe"

// readClipboard reads text from the system clipboard via X11.
func readClipboard() string {
	cstr := C.readX11Clipboard()
	if cstr == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(cstr))
	return C.GoString(cstr)
}
