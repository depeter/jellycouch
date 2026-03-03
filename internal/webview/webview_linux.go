//go:build linux

package webview

/*
#cgo pkg-config: gtk+-3.0 webkit2gtk-4.1

#include <gtk/gtk.h>
#include <webkit2/webkit2.h>

static void run_webview(const char *url, const char *init_js) {
    gtk_init(0, NULL);

    GtkWidget *window = gtk_window_new(GTK_WINDOW_TOPLEVEL);
    gtk_window_set_title(GTK_WINDOW(window), "JellyCouch Web");
    gtk_window_set_default_size(GTK_WINDOW(window), 1920, 1080);

    // Create user content manager for JS injection
    WebKitUserContentManager *ucm = webkit_user_content_manager_new();

    if (init_js && init_js[0]) {
        WebKitUserScript *script = webkit_user_script_new(
            init_js,
            WEBKIT_USER_CONTENT_INJECT_ALL_FRAMES,
            WEBKIT_USER_SCRIPT_INJECT_AT_DOCUMENT_END,
            NULL, NULL
        );
        webkit_user_content_manager_add_script(ucm, script);
        webkit_user_script_unref(script);
    }

    GtkWidget *webview = webkit_web_view_new_with_user_content_manager(ucm);
    gtk_container_add(GTK_CONTAINER(window), webview);

    g_signal_connect(window, "destroy", G_CALLBACK(gtk_main_quit), NULL);

    gtk_window_fullscreen(GTK_WINDOW(window));
    gtk_widget_show_all(window);

    webkit_web_view_load_uri(WEBKIT_WEB_VIEW(webview), url);

    gtk_main();
}
*/
import "C"

import (
	"os"
	"os/exec"
	"unsafe"
)

// RunWebview creates a fullscreen WebKitGTK webview with spatial navigation.
// This blocks until the webview window is closed.
func RunWebview(url string) {
	// Build the init JS: define __jc_close as window.close, then add spatial nav
	closeJS := `window.__jc_close = function() { window.close(); };`
	initJS := closeJS + "\n" + spatialNavJS

	curl := C.CString(url)
	defer C.free(unsafe.Pointer(curl))
	cjs := C.CString(initJS)
	defer C.free(unsafe.Pointer(cjs))

	C.run_webview(curl, cjs)
}

// StartWebApp launches the current executable with --webview <url> as a child
// process. The webview runs in a separate OS process to avoid conflicts between
// Ebitengine (GLFW) and WebKitGTK (GTK) event loops.
func StartWebApp(url string) (*exec.Cmd, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(exe, "--webview", url)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}
