// Package logger owns structured logging via log/slog with event enums and RFC 3339 nanosecond timestamps.
//
// Phase 1 implements the real subsystem — Initialize wires log/slog.JSONHandler
// to an append-mode file with a custom ReplaceAttr that writes fixed 9-digit
// RFC 3339 nanosecond timestamps (the stdlib's time.RFC3339Nano trims trailing
// zeros, which would break log-parsing harnesses that assume constant-width
// fractional seconds per architecture §Structured Log Event Schema).
package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

// rfc3339FixedNano is time.RFC3339Nano with `9`s replaced by `0`s to force a
// constant-width 9-digit fractional-second field. Architecture §Structured Log
// Event Schema example shows nine digits, and the verification regex in Phase 1
// Step 3 asserts exactly nine.
const rfc3339FixedNano = "2006-01-02T15:04:05.000000000Z07:00"

var (
	currentFileMu sync.Mutex
	currentFile   *os.File
)

// Initialize wires log/slog with a JSON handler writing to logPath in append
// mode and installs it as the default slog handler. `level` controls which
// records are emitted; records below the level are filtered at handler time.
//
// The parent directory of logPath is auto-created via os.MkdirAll. NTFS mode
// bits 0o755/0o644 are silently ignored on Windows (per security-plan §Stack-
// Specific Bans — real protection comes from inherited NTFS ACLs).
//
// If Initialize is called more than once, the previous file handle is closed
// before the new one opens. Tests that exercise Initialize on a t.TempDir path
// should call Close at t.Cleanup to release the handle before TempDir removal
// runs (Windows refuses to delete a directory containing open file handles).
func Initialize(level slog.Level, logPath string) error {
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return fmt.Errorf("initialize slog handler (logPath=%s): %w", logPath, err)
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("initialize slog handler (logPath=%s): %w", logPath, err)
	}
	currentFileMu.Lock()
	if currentFile != nil {
		_ = currentFile.Close()
	}
	currentFile = f
	currentFileMu.Unlock()
	return InitializeWithWriter(level, f)
}

// InitializeWithWriter configures slog to write JSON records to w at the given
// level. Exposed for tests that use an in-memory bytes.Buffer instead of a
// real file; production paths go through Initialize.
func InitializeWithWriter(level slog.Level, w io.Writer) error {
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey && a.Value.Kind() == slog.KindTime {
				a.Value = slog.StringValue(a.Value.Time().Format(rfc3339FixedNano))
			}
			return a
		},
	})
	slog.SetDefault(slog.New(handler))
	return nil
}

// Close releases the log file handle if Initialize was called with a file path.
// Safe to call multiple times; no-op if the handle is already nil.
func Close() error {
	currentFileMu.Lock()
	defer currentFileMu.Unlock()
	if currentFile == nil {
		return nil
	}
	err := currentFile.Close()
	currentFile = nil
	return err
}
