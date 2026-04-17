// Package toast owns Windows toast-notification dispatch via the upstream
// go-toast/toast library, plus first-run AppUserModelID registration in
// HKCU\Software\Classes\AppUserModelId\Turbolet85.ClipClap. Phase 3
// implements the real subsystem; Phase 0 stubs the package so the directory
// exists and `go build ./...` succeeds.
package toast

// Initialize is a placeholder; Phase 3 replaces it with the real toast
// subsystem entry point.
func Initialize() error { return nil }
