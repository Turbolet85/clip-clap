// Package hotkey owns global hotkey registration via Win32 RegisterHotKey
// and dispatches WM_HOTKEY through the shared message pump. Phase 2
// implements the registration + parser; Phase 0 stubs the package so the
// directory exists and `go build ./...` succeeds.
package hotkey

// Initialize is a placeholder; Phase 2 replaces it with the real hotkey
// subsystem entry point.
func Initialize() error { return nil }
