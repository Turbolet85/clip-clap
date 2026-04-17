// Package overlay owns the transparent full-screen WS_EX_LAYERED window used
// for area-select drag capture. Phase 3 implements the layered-window draw
// loop, drag-rectangle compositing, and capture trigger; Phase 0 stubs the
// package so the directory exists and `go build ./...` succeeds.
package overlay

// Initialize is a placeholder; Phase 3 replaces it with the real overlay
// subsystem entry point.
func Initialize() error { return nil }
