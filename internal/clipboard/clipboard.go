// Package clipboard owns Win32 OpenClipboard / SetClipboardData(CF_UNICODETEXT)
// writes plus the in-memory Undo snapshot and the per-capture 500ms reentry
// guard. Phase 3 implements the real subsystem; Phase 0 stubs the package so
// the directory exists and `go build ./...` succeeds.
package clipboard

// Initialize is a placeholder; Phase 3 replaces it with the real clipboard
// subsystem entry point.
func Initialize() error { return nil }
