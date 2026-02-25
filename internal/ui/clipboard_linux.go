//go:build linux || freebsd || netbsd || openbsd

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

// writeX11Clipboard takes ownership of the CLIPBOARD selection and serves
// the provided text to one paste request, then exits. This runs in a
// forked process-like manner (separate thread with its own Display connection)
// so the main thread is not blocked. The data remains available until
// another application takes ownership of the clipboard.
//
// Because X11 clipboard ownership requires an event loop, we serve
// requests for up to 30 seconds or until someone else claims the selection.
static void writeX11Clipboard(const char *text, int textLen) {
    Display *dpy = XOpenDisplay(NULL);
    if (!dpy) return;

    Window win = XCreateSimpleWindow(dpy, DefaultRootWindow(dpy), 0, 0, 1, 1, 0, 0, 0);
    Atom clipboard = XInternAtom(dpy, "CLIPBOARD", False);
    Atom utf8str = XInternAtom(dpy, "UTF8_STRING", False);
    Atom targets = XInternAtom(dpy, "TARGETS", False);

    XSetSelectionOwner(dpy, clipboard, win, CurrentTime);
    XFlush(dpy);

    if (XGetSelectionOwner(dpy, clipboard) != win) {
        XDestroyWindow(dpy, win);
        XCloseDisplay(dpy);
        return;
    }

    struct timespec start, now;
    clock_gettime(CLOCK_MONOTONIC, &start);

    while (1) {
        clock_gettime(CLOCK_MONOTONIC, &now);
        double elapsed = (now.tv_sec - start.tv_sec) + (now.tv_nsec - start.tv_nsec) / 1e9;
        if (elapsed > 30.0) break;

        if (XPending(dpy) > 0) {
            XEvent ev;
            XNextEvent(dpy, &ev);

            if (ev.type == SelectionClear) {
                // Another app took ownership
                break;
            }

            if (ev.type == SelectionRequest) {
                XSelectionRequestEvent *req = &ev.xselectionrequest;
                XSelectionEvent resp;
                memset(&resp, 0, sizeof(resp));
                resp.type = SelectionNotify;
                resp.requestor = req->requestor;
                resp.selection = req->selection;
                resp.target = req->target;
                resp.time = req->time;
                resp.property = None;

                if (req->target == targets) {
                    Atom supported[] = {targets, utf8str, XA_STRING};
                    XChangeProperty(dpy, req->requestor,
                        req->property ? req->property : req->target,
                        XA_ATOM, 32, PropModeReplace,
                        (unsigned char*)supported, 3);
                    resp.property = req->property ? req->property : req->target;
                } else if (req->target == utf8str || req->target == XA_STRING) {
                    XChangeProperty(dpy, req->requestor,
                        req->property ? req->property : req->target,
                        utf8str, 8, PropModeReplace,
                        (unsigned char*)text, textLen);
                    resp.property = req->property ? req->property : req->target;
                }

                XSendEvent(dpy, req->requestor, False, 0, (XEvent*)&resp);
                XFlush(dpy);
            }
        } else {
            struct timespec ts = {0, 50000000}; // 50ms
            nanosleep(&ts, NULL);
        }
    }

    XDestroyWindow(dpy, win);
    XCloseDisplay(dpy);
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

// writeClipboard writes text to the system clipboard via X11.
// The clipboard content is served in a background goroutine for up to 30 seconds.
func writeClipboard(text string) {
	cstr := C.CString(text)
	clen := C.int(len(text))
	go func() {
		C.writeX11Clipboard(cstr, clen)
		C.free(unsafe.Pointer(cstr))
	}()
}
