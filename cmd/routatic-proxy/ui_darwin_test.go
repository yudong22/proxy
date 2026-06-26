//go:build darwin

package main

// These tests guard against regressions in the close-button behavior. The
// earlier implementation called wv.Destroy() in goWindowWillClose, which
// tore down the WKWebView and (transitively) called stop_run_loop() on the
// NSApp run loop. The system tray depends on that run loop, so the whole
// tray icon vanished whenever the user clicked the red close button.
//
// The fix is a CGO swizzle in ui_darwin.go that overrides windowShouldClose:
// on the webview's window delegate to return NO after ordering the window
// out. These tests verify the Go side of that contract:

import (
	"strings"
	"testing"
)

// TestUIFile_NoStaleCloseObserver ensures the old NSWindowWillCloseNotification
// observer (which used to call goWindowWillClose → wv.Destroy) has been
// removed. If somebody reintroduces it, the system tray will start vanishing
// on close again.
func TestUIFile_NoStaleCloseObserver(t *testing.T) {
	src := readFile(t, "ui_darwin.go")
	for _, needle := range []string{
		"NSWindowWillCloseNotification",
		"goWindowWillClose",
		"registerWindowCloseObserver",
	} {
		if strings.Contains(src, needle) {
			t.Errorf("ui_darwin.go still references %q — this old path kills the system tray on close", needle)
		}
	}
}

// TestUIFile_NoDestroyOnClose ensures that no code path in the close-handling
// chain calls wv.Destroy(). Destroy() unwinds the WKWebView and terminates the
// NSApp run loop, which is what causes the system tray to disappear.
func TestUIFile_NoDestroyOnClose(t *testing.T) {
	src := readFile(t, "ui_darwin.go")
	// The only legitimate use of Destroy() would be on a hard-exit path
	// (e.g. when the process is truly quitting). We expect ZERO references
	// in the current implementation.
	if strings.Contains(src, "wv.Destroy()") && !strings.Contains(src, "// wv.Destroy()") {
		// Look for the actual call site, not a comment. Search for the
		// function call shape with the dot-call pattern.
		idx := strings.Index(src, "wv.Destroy()")
		// Allow a comment marker (we don't expect any) but the live source
		// must not contain it.
		if idx >= 0 {
			// Look back a few chars for a `//` comment marker.
			lineStart := strings.LastIndex(src[:idx], "\n") + 1
			line := src[lineStart:idx]
			if !strings.HasPrefix(strings.TrimSpace(line), "//") {
				t.Errorf("ui_darwin.go contains wv.Destroy() at offset %d — this terminates the NSApp run loop and kills the system tray", idx)
			}
		}
	}
}

// TestUIFile_HasHideOnCloseSwizzle ensures the replacement (a swizzle that
// hides the window instead of closing it) is present.
func TestUIFile_HasHideOnCloseSwizzle(t *testing.T) {
	src := readFile(t, "ui_darwin.go")
	if !strings.Contains(src, "windowShouldClose") {
		t.Error("ui_darwin.go is missing the windowShouldClose: swizzle that hides the window on close")
	}
	if !strings.Contains(src, "orderOut") {
		t.Error("ui_darwin.go is missing orderOut: call — the close button must hide the window, not close it")
	}
}
