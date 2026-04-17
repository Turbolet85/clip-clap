// Package status owns the loopback-only HTTP /status endpoint exposed at
// 127.0.0.1:27773 when the binary is launched with --agent-mode (default
// off). Phase 4 implements the real net/http handler with Host-header
// allowlist and Origin-reject middleware; Phase 0 stubs the package so the
// directory exists and `go build ./...` succeeds.
package status

// Initialize is a placeholder; Phase 4 replaces it with the real status
// endpoint registration.
func Initialize() error { return nil }
