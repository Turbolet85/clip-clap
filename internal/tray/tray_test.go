package tray

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"
)

// TestMenuIDMapping pins the six WM_COMMAND dispatch ids. These numbers
// travel across Win32's 16-bit wparam and land in MenuIDToName via WndProc;
// reassigning any value silently breaks menu dispatch so every test
// asserts the exact numeric value, not just distinctness.
func TestMenuIDMapping(t *testing.T) {
	cases := []struct {
		name string
		got  int
		want int
	}{
		{"MenuIDCapture", MenuIDCapture, 1},
		{"MenuIDOpenFolder", MenuIDOpenFolder, 2},
		{"MenuIDSettings", MenuIDSettings, 3},
		{"MenuIDQuit", MenuIDQuit, 4},
		{"MenuIDUndoLastCapture", MenuIDUndoLastCapture, 5},
		{"MenuIDLastError", MenuIDLastError, 6},
	}
	seen := map[int]string{}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.got != c.want {
				t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
			}
			if c.got <= 0 {
				t.Errorf("%s = %d, want positive", c.name, c.got)
			}
			if prior, dup := seen[c.got]; dup {
				t.Errorf("%s = %d collides with %s", c.name, c.got, prior)
			}
			seen[c.got] = c.name
		})
	}
}

// TestMenuIDToName_KnownIDs ensures every menu ID constant maps to a
// stable, machine-parseable name — used by log messages and future
// Phase 4 pytest assertions. Unknown IDs fall through to "unknown".
func TestMenuIDToName_KnownIDs(t *testing.T) {
	cases := []struct {
		id   int
		want string
	}{
		{MenuIDCapture, "capture"},
		{MenuIDOpenFolder, "open_folder"},
		{MenuIDSettings, "settings"},
		{MenuIDQuit, "quit"},
		{MenuIDUndoLastCapture, "undo_last_capture"},
		{MenuIDLastError, "last_error"},
		{999, "unknown"},
	}
	for _, c := range cases {
		if got := MenuIDToName(c.id); got != c.want {
			t.Errorf("MenuIDToName(%d) = %q, want %q", c.id, got, c.want)
		}
	}
}

// TestSanitizeForTray_NilUnchanged ensures the helper is safe to wrap
// around any error, even a nil one, without introducing a nil-check at
// every call site.
func TestSanitizeForTray_NilUnchanged(t *testing.T) {
	if got := SanitizeForTray(nil); got != nil {
		t.Errorf("SanitizeForTray(nil) = %v, want nil", got)
	}
}

// TestSanitizeForTray_PlainErrorUnchanged verifies that non-path errors
// pass through without modification — the helper does not paper over
// arbitrary errors, only path-bearing ones.
func TestSanitizeForTray_PlainErrorUnchanged(t *testing.T) {
	orig := errors.New("something went wrong")
	got := SanitizeForTray(orig)
	if got.Error() != orig.Error() {
		t.Errorf("SanitizeForTray(%q) = %q, want unchanged", orig, got)
	}
}

// TestSanitizeForTray_PathErrorRedacted is the security gate from
// security-plan §Error Handling: `*os.PathError` wraps a full file path
// that would leak the user's home dir in screenshot/screen-shares if
// surfaced to the tray. The sanitized result must retain the Op and
// underlying Err but strip the path to its basename.
func TestSanitizeForTray_PathErrorRedacted(t *testing.T) {
	orig := &os.PathError{
		Op:   "open",
		Path: `C:\Users\alice\AppData\Local\secret\config.toml`,
		Err:  errors.New("access is denied"),
	}
	sanitized := SanitizeForTray(orig)
	msg := sanitized.Error()
	if strings.Contains(msg, `C:\Users\alice`) {
		t.Errorf("SanitizeForTray leaked full path: %q", msg)
	}
	if !strings.Contains(msg, "config.toml") {
		t.Errorf("SanitizeForTray dropped basename: %q (expected 'config.toml' present)", msg)
	}
	if !strings.Contains(msg, "access is denied") {
		t.Errorf("SanitizeForTray dropped underlying message: %q", msg)
	}
}

// TestSanitizeForTray_FSPathErrorRedacted covers the fs.PathError path
// (Go 1.16+ alias of os.PathError) so a future stdlib decoupling does
// not silently leak paths from io/fs-returning code.
func TestSanitizeForTray_FSPathErrorRedacted(t *testing.T) {
	orig := &fs.PathError{
		Op:   "read",
		Path: `D:\secrets\notes.txt`,
		Err:  errors.New("permission denied"),
	}
	sanitized := SanitizeForTray(orig)
	msg := sanitized.Error()
	if strings.Contains(msg, `D:\secrets`) {
		t.Errorf("SanitizeForTray leaked fs path: %q", msg)
	}
	if !strings.Contains(msg, "notes.txt") {
		t.Errorf("SanitizeForTray dropped basename: %q", msg)
	}
}
