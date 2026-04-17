// Package tray owns the Windows system-tray surface (Shell_NotifyIcon icon,
// TrackPopupMenuEx context menu, tooltip, and the 350ms safelight capture
// flash). Phase 1+ provides the real implementation; Phase 0 ships only this
// stub so the package directory exists and `go build ./...` succeeds.
package tray

// Initialize is a placeholder so the package contributes one exported symbol;
// Phase 1+ replaces this with the real subsystem entry point.
func Initialize() error { return nil }
