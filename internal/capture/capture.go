// Package capture orchestrates the screen-capture pipeline: kbinani/screenshot
// CaptureRect, PNG encode, save_folder ensure-exists, filename formatting.
// Phase 3 implements the real pipeline; Phase 0 stubs the package so the
// directory exists and `go build ./...` succeeds.
package capture

// Initialize is a placeholder; Phase 3 replaces it with the real capture
// subsystem entry point.
func Initialize() error { return nil }
