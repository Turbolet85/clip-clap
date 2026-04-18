package status

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestWritePIDFile_Format verifies AC #3: `.agent-running` contains
// exactly `<decimal digits>\n` (the process PID + one newline), no
// JSON braces, no extra whitespace.
func TestWritePIDFile_Format(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".agent-running")

	if err := WritePIDFile(path); err != nil {
		t.Fatalf("WritePIDFile: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	want := strconv.Itoa(os.Getpid()) + "\n"
	if string(got) != want {
		t.Errorf("PID file contents = %q, want %q (exactly decimal PID + one newline)", string(got), want)
	}

	// Extra defensive assertions to make failure messages clear.
	if strings.Contains(string(got), "{") || strings.Contains(string(got), "}") {
		t.Errorf("PID file must not contain JSON braces, got %q", string(got))
	}
	if strings.Count(string(got), "\n") != 1 {
		t.Errorf("PID file must have exactly one newline, got %d in %q", strings.Count(string(got), "\n"), string(got))
	}
}

// TestReadPIDFile_IgnoresTrailingNewline validates round-trip parsing.
func TestReadPIDFile_IgnoresTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".agent-running")
	if err := os.WriteFile(path, []byte("7331\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	got, err := ReadPIDFile(path)
	if err != nil {
		t.Fatalf("ReadPIDFile: %v", err)
	}
	if got != 7331 {
		t.Errorf("ReadPIDFile = %d, want 7331", got)
	}
}

// TestReadPIDFile_RejectsJSON verifies the strict "single decimal
// integer" contract from the harness. JSON content (like `{"pid":5240}`)
// MUST be rejected with an error whose message contains the substring
// "non-integer" — Step 2's Security: note pins this wording.
func TestReadPIDFile_RejectsJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".agent-running")
	if err := os.WriteFile(path, []byte(`{"pid":5240}`), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	_, err := ReadPIDFile(path)
	if err == nil {
		t.Fatal("ReadPIDFile on JSON input returned nil error, want non-nil")
	}
	if !strings.Contains(err.Error(), "non-integer") {
		t.Errorf("ReadPIDFile error = %q, want substring %q", err.Error(), "non-integer")
	}
}

// TestDeletePIDFile_Idempotent validates the `os.IsNotExist` tolerance
// from Step 2. Deleting a missing file must not error — this mirrors
// the PowerShell harness's `Remove-Item -ErrorAction SilentlyContinue`.
func TestDeletePIDFile_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.pid")

	if err := DeletePIDFile(path); err != nil {
		t.Errorf("DeletePIDFile on missing file returned err %v, want nil", err)
	}

	// Also test the happy path: create, delete, assert gone.
	if err := os.WriteFile(path, []byte("123\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := DeletePIDFile(path); err != nil {
		t.Errorf("DeletePIDFile on existing file returned err %v, want nil", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("PID file still exists after DeletePIDFile, stat err = %v", err)
	}

	// Double-delete must also be a no-op.
	if err := DeletePIDFile(path); err != nil {
		t.Errorf("second DeletePIDFile returned err %v, want nil", err)
	}
}

// guard against accidentally importing fmt in non-test builds if state.go
// ever gets streamlined — keeps the import used in the test file only.
var _ = fmt.Errorf
