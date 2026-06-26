//go:build darwin

package main

import (
	"os"
	"testing"
)

// readFile reads a file relative to this source file's directory. Tests
// use it to inspect the source of nearby files (e.g. ui_darwin.go) for
// regression-prone patterns without invoking the CGO build.
func readFile(t *testing.T, name string) string {
	t.Helper()
	// The test binary runs with cwd = package dir for `go test ./...`,
	// but we resolve relative to the source file's directory to be safe.
	paths := []string{name, "../" + name}
	for _, p := range paths {
		if b, err := os.ReadFile(p); err == nil {
			return string(b)
		}
	}
	t.Fatalf("readFile: could not locate %s", name)
	return ""
}
