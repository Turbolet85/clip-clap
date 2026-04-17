package tray

import "fmt"

// tooltipIdle is the steady-state tooltip string — Phase 2 uses this at
// all times; Phase 3 will extend BuildTooltip to show "Last: <filename>"
// after a successful capture.
const tooltipIdle = "clip-clap"

// lastErrorMaxLen is the max length for error-message display in the
// tray "Last error" menu slot. Exceeding this pushes the menu outside
// reasonable bounds and obscures the other menu items — truncate to
// keep the drawer a fixed width. 80 chars approximates the Win11 dark
// menu's visual budget before text clips.
const lastErrorMaxLen = 80

// BuildTooltip returns the string passed into NOTIFYICONDATA.szTip (max
// 127 UTF-16 chars per Win32). Phase 2 returns a fixed "clip-clap"; the
// live filename display is a Phase 3 extension so this function signature
// can grow without breaking callers.
func BuildTooltip() string {
	return tooltipIdle
}

// FormatLastErrorMenuLabel renders the "Last error: …" menu label. Nil
// error yields "<none>"; non-nil error's message is truncated to
// lastErrorMaxLen chars with a trailing "…" so the menu drawer stays
// readable when a long stack trace or path slips through.
//
// Callers must sanitize the error (strip full paths) BEFORE passing it
// in — SanitizeForTray in tray.go is the expected preprocessor. This
// function is pure and does not touch the file system or lasterror
// package; anti-pattern enforcement (no motion, no custom fonts) is
// enforced in the Win32 tray code, not here.
func FormatLastErrorMenuLabel(err error) string {
	if err == nil {
		return "Last error: <none>"
	}
	msg := err.Error()
	if len(msg) > lastErrorMaxLen {
		// -1 to reserve one byte for the ellipsis. ASCII "..." instead of
		// U+2026 "…" so the label stays plain-ASCII-safe through every
		// Win32 menu codepath (even though szTip is UTF-16, consistency
		// with other surfaces beats the 2-byte saving).
		msg = msg[:lastErrorMaxLen-3] + "..."
	}
	return fmt.Sprintf("Last error: %s", msg)
}
