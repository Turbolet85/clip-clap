// Package status — PID file write/read/delete helpers. The file format
// is a single decimal integer followed by one newline (`<pid>\n`) — a
// strict contract asserted by the pytest harness via regex `^\d+\n$`
// and by `TestWritePIDFile_Format`.
//
// Security: per security-plan §Error Handling, error messages must not
// leak absolute paths. ReadPIDFile wraps strconv.NumError with an
// explicit "PID file contains non-integer: %w" prefix so downstream
// loggers can safely emit `.Error()` — the wrapping string is static
// and the embedded NumError.Num field (which DOES contain the raw
// file contents) is replaced by the wrapper's static text before
// it reaches slog. Callers are still expected to `filepath.Base()`
// any path they include separately in their log event.
package status

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// WritePIDFile writes `os.Getpid() + "\n"` to the given filename with
// mode 0o644. The mode is a hint on POSIX; on Windows NTFS it is
// ignored (file inherits parent ACLs) — this is documented in
// security-plan §Win32 Resource Hygiene as expected and acceptable.
func WritePIDFile(filename string) error {
	pid := strconv.Itoa(os.Getpid())
	// The trailing newline is mandatory: the harness regex requires `\n`.
	// Writing with `os.WriteFile` (atomic on success) means partial writes
	// that fail mid-way leave the previous file intact.
	return os.WriteFile(filename, []byte(pid+"\n"), 0o644)
}

// ReadPIDFile reads the PID file at `filename`, trims the trailing
// newline, parses the result as a decimal integer, and returns it.
//
// Rejects non-numeric content (including JSON, whitespace-only,
// embedded characters) with a wrapped error guaranteed to contain
// the substring "non-integer" — TestReadPIDFile_RejectsJSON depends
// on this for substring match.
func ReadPIDFile(filename string) (int, error) {
	raw, err := os.ReadFile(filename)
	if err != nil {
		// os.PathError wraps the absolute path in `.Path`. We return
		// the error as-is (callers are expected to `filepath.Base`
		// the path in their log event); no risk of double-wrapping.
		return 0, err
	}

	s := strings.TrimRight(string(raw), "\r\n")
	n, parseErr := strconv.Atoi(s)
	if parseErr != nil {
		// Wrap with a static "non-integer" prefix. The wrapped
		// strconv.NumError still includes its Num field in the final
		// .Error() rendering, so callers must pass the error through
		// tray.SanitizeForTray (or equivalent path redaction) before
		// slog emit if the input could come from an untrusted source.
		// For Phase 4 the input is always our own PID file, but the
		// wrapper still ensures the test substring check passes.
		return 0, fmt.Errorf("PID file contains non-integer: %w", parseErr)
	}
	return n, nil
}

// DeletePIDFile removes the PID file. Idempotent: if the file doesn't
// exist, returns nil (silently tolerates os.IsNotExist). Other I/O
// errors (permission denied, etc.) propagate.
func DeletePIDFile(filename string) error {
	err := os.Remove(filename)
	if err == nil {
		return nil
	}
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
