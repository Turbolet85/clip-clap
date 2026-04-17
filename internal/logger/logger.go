// Package logger owns the stdlib log/slog JSONHandler setup and the event-key
// constants enumerated in architecture §Standard Contracts. Named `logger`
// (not `log`) to avoid shadowing the stdlib `log` package — see
// docs/dependencies.md for the architecture-compliance deviation note.
// Phase 1 implements the real subsystem; Phase 0 stubs the package so the
// directory exists and `go build ./...` succeeds.
package logger

// Initialize is a placeholder; Phase 1 replaces it with the real logger
// setup entry point that wires log/slog JSONHandler to logs/agent-latest.jsonl.
func Initialize() error { return nil }
