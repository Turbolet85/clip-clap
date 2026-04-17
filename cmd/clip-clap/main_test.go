package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestVersionFlag_PrintsLiteral asserts run() with --version writes exactly
// "clip-clap v0.0.1\n" to stdout (canonical fmt.Fprintln output) and exits 0.
// This is the unit-level enforcement of AC #4 from the Phase 0 plan.
func TestVersionFlag_PrintsLiteral(t *testing.T) {
	var buf bytes.Buffer
	code := run([]string{"--version"}, &buf)

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	const want = "clip-clap v0.0.1\n"
	if got := buf.String(); got != want {
		t.Errorf("stdout mismatch:\n  want: %q\n  got:  %q", want, got)
	}
}

// TestNoArgs_HasNoVersionOutput asserts run() with no arguments does NOT
// print the version string. Proves --version is the gate, not default
// behavior. Phase 0 skeleton's no-args path is a no-op exit (return 0);
// Phase 2+ replaces this with the message-pump startup.
func TestNoArgs_HasNoVersionOutput(t *testing.T) {
	var buf bytes.Buffer
	code := run([]string{}, &buf)

	if code != 0 {
		t.Errorf("expected no-args exit code 0 (Phase 0 skeleton no-op), got %d", code)
	}
	if got := buf.String(); strings.Contains(got, "v0.0.1") {
		t.Errorf("stdout unexpectedly contains version string: %q", got)
	}
}
