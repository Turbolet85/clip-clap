package tray

import (
	"errors"
	"strings"
	"testing"
)

// TestBuildTooltip_ReturnsClipClap pins the NOTIFYICONDATA.szTip content.
// AC #2 asserts on the literal "clip-clap" string hovering the tray
// icon — changing this return value silently breaks the acceptance test.
func TestBuildTooltip_ReturnsClipClap(t *testing.T) {
	if got := BuildTooltip(); got != "clip-clap" {
		t.Errorf("BuildTooltip() = %q, want %q", got, "clip-clap")
	}
}

// TestFormatLastErrorMenuLabel_NilError is the idle-state Last error
// slot — Win11 renders this grayed menu item as "Last error: <none>"
// per the design brief.
func TestFormatLastErrorMenuLabel_NilError(t *testing.T) {
	if got := FormatLastErrorMenuLabel(nil); got != "Last error: <none>" {
		t.Errorf("FormatLastErrorMenuLabel(nil) = %q, want %q", got, "Last error: <none>")
	}
}

// TestFormatLastErrorMenuLabel_WithError covers the short-error path
// where the error message fits within lastErrorMaxLen — no truncation,
// no ellipsis.
func TestFormatLastErrorMenuLabel_WithError(t *testing.T) {
	err := errors.New("boom")
	want := "Last error: boom"
	if got := FormatLastErrorMenuLabel(err); got != want {
		t.Errorf("FormatLastErrorMenuLabel(boom) = %q, want %q", got, want)
	}
}

// TestFormatLastErrorMenuLabel_LongErrorTruncation enforces the
// lastErrorMaxLen budget. Long errors (e.g., full stack traces passed
// through recover()) must fit the Win11 menu drawer — truncate to max
// chars with a trailing ellipsis so the user sees a stable UI width.
func TestFormatLastErrorMenuLabel_LongErrorTruncation(t *testing.T) {
	longMsg := strings.Repeat("a", 200)
	err := errors.New(longMsg)
	got := FormatLastErrorMenuLabel(err)
	// "Last error: " prefix + up to lastErrorMaxLen (80) chars of message.
	maxLen := len("Last error: ") + lastErrorMaxLen
	if len(got) > maxLen {
		t.Errorf("label length %d > budget %d: %q", len(got), maxLen, got)
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("long message must end with '...' ellipsis, got %q", got)
	}
}
