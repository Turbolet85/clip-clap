// Package hotkey owns global-hotkey registration via Win32 RegisterHotKey
// and the allowlist-based parser for user-configured hotkey strings like
// "Ctrl+Shift+S". The package is Windows-only (per architecture §Stack —
// "direct Win32 via golang.org/x/sys/windows"). It exposes two entry
// points: ParseHotkeyString (Step 2 — pure) and Register (Step 7 — Win32).
package hotkey

import (
	"fmt"
	"log/slog"
	"strings"

	"golang.org/x/sys/windows"

	"github.com/Turbolet85/clip-clap/internal/lasterror"
	"github.com/Turbolet85/clip-clap/internal/logger"
)

// procRegisterHotKey is the lazy-loaded Win32 entry point for global hotkey
// binding. golang.org/x/sys/windows v0.24.0 does not wrap RegisterHotKey,
// so we resolve it from user32.dll at first use. Package-level scope so
// multiple Register() calls share one proc pointer (no per-call resolution
// cost).
var (
	user32             = windows.NewLazySystemDLL("user32.dll")
	procRegisterHotKey = user32.NewProc("RegisterHotKey")
)

// Win32 modifier flags accepted by RegisterHotKey's fsModifiers argument.
// golang.org/x/sys/windows does not expose MOD_* constants as of v0.24.0,
// so we define them locally with the Win32-documented values. No dynamic
// lookup — these are a fixed allowlist enforced in modTokenMap below.
const (
	MOD_ALT     uint32 = 0x0001
	MOD_CONTROL uint32 = 0x0002
	MOD_SHIFT   uint32 = 0x0004
	MOD_WIN     uint32 = 0x0008
)

// Win32 virtual-key codes (Win32 User Input Reference). Letter keys map
// directly to their ASCII uppercase code (e.g., 'S' = 0x53 = VK_S per
// convention); number keys likewise ('0'–'9' = 0x30–0x39). Named keys
// (F1, Space, PageUp, etc.) require explicit constants since x/sys does
// not export them. Only the subset exercised by the Phase 2 allowlist is
// declared — extending the hotkey surface is a v1.x enhancement.
const (
	VK_SPACE  uint32 = 0x20
	VK_PRIOR  uint32 = 0x21 // PageUp
	VK_NEXT   uint32 = 0x22 // PageDown
	VK_END    uint32 = 0x23
	VK_HOME   uint32 = 0x24
	VK_INSERT uint32 = 0x2D
	VK_DELETE uint32 = 0x2E
	VK_F1     uint32 = 0x70
	VK_F2     uint32 = 0x71
	VK_F3     uint32 = 0x72
	VK_F4     uint32 = 0x73
	VK_F5     uint32 = 0x74
	VK_F6     uint32 = 0x75
	VK_F7     uint32 = 0x76
	VK_F8     uint32 = 0x77
	VK_F9     uint32 = 0x78
	VK_F10    uint32 = 0x79
	VK_F11    uint32 = 0x7A
	VK_F12    uint32 = 0x7B
)

// modTokenMap is the allowlist for modifier tokens — security-plan §Input
// Validation mandates an allowlist parser over dynamic ToUpper/ToLower
// normalization. Keys are case-sensitive; "CTRL" and "ctrl" are rejected
// so config typos surface loudly rather than silently accepting variants.
var modTokenMap = map[string]uint32{
	"Ctrl":  MOD_CONTROL,
	"Shift": MOD_SHIFT,
	"Alt":   MOD_ALT,
	"Win":   MOD_WIN,
}

// vkTokenMap is the allowlist for virtual-key tokens. Named keys are
// listed here; single-letter and single-digit tokens are handled inline
// in parseVK below (since A–Z/0–9 is a well-defined contiguous range).
var vkTokenMap = map[string]uint32{
	"Space":    VK_SPACE,
	"PageUp":   VK_PRIOR,
	"PageDown": VK_NEXT,
	"Home":     VK_HOME,
	"End":      VK_END,
	"Insert":   VK_INSERT,
	"Delete":   VK_DELETE,
	"F1":       VK_F1,
	"F2":       VK_F2,
	"F3":       VK_F3,
	"F4":       VK_F4,
	"F5":       VK_F5,
	"F6":       VK_F6,
	"F7":       VK_F7,
	"F8":       VK_F8,
	"F9":       VK_F9,
	"F10":      VK_F10,
	"F11":      VK_F11,
	"F12":      VK_F12,
}

// ParseHotkeyString parses a hotkey config string like "Ctrl+Shift+S"
// into the (mods, vk) pair expected by RegisterHotKey. The grammar is
// a `+`-separated token list where every token except the last must be
// a modifier (see modTokenMap) and the last must be a key: either a
// named key from vkTokenMap, an ASCII letter A–Z, or an ASCII digit 0–9.
//
// Case-sensitivity is enforced — "CTRL+Shift+S" returns an error — so
// config typos cannot silently re-interpret the user's hotkey. Return
// on first invalid token (fail-fast) so the error message points at
// the offending segment rather than a downstream consequence.
//
// Pure function: no Win32 calls, no slog, no global state mutation.
func ParseHotkeyString(s string) (mods uint32, vk uint32, err error) {
	if s == "" {
		return 0, 0, fmt.Errorf("hotkey: empty string")
	}
	tokens := strings.Split(s, "+")
	if len(tokens) < 2 {
		return 0, 0, fmt.Errorf("hotkey: %q has no modifier+key pair", s)
	}
	// All tokens except the last must be modifiers.
	for i := 0; i < len(tokens)-1; i++ {
		tok := tokens[i]
		if tok == "" {
			return 0, 0, fmt.Errorf("hotkey: empty modifier token in %q", s)
		}
		mod, ok := modTokenMap[tok]
		if !ok {
			return 0, 0, fmt.Errorf("hotkey: unknown modifier %q in %q", tok, s)
		}
		mods |= mod
	}
	// Last token must be a key.
	keyTok := tokens[len(tokens)-1]
	if keyTok == "" {
		return 0, 0, fmt.Errorf("hotkey: empty key token in %q", s)
	}
	vk, err = parseVK(keyTok)
	if err != nil {
		return 0, 0, fmt.Errorf("hotkey: %w", err)
	}
	return mods, vk, nil
}

// parseVK resolves a single key token to its Win32 virtual-key code.
// Precedence: named-key lookup in vkTokenMap, then single-letter A–Z
// (ASCII uppercase maps directly to VK_A..VK_Z), then single-digit 0–9
// (ASCII digit maps directly to VK_0..VK_9). Anything else is rejected.
func parseVK(tok string) (uint32, error) {
	if vk, ok := vkTokenMap[tok]; ok {
		return vk, nil
	}
	if len(tok) == 1 {
		c := tok[0]
		if c >= 'A' && c <= 'Z' {
			return uint32(c), nil
		}
		if c >= '0' && c <= '9' {
			return uint32(c), nil
		}
	}
	return 0, fmt.Errorf("unknown key %q (expected A-Z, 0-9, or a named key like F1/Space/PageUp)", tok)
}

// Register parses hotkeyStr via ParseHotkeyString, then calls Win32
// RegisterHotKey(hwnd, id, mods, vk). On either parse failure or Win32
// failure, the error is published to lasterror (so the tray "Last error"
// slot reflects it) and emitted as a logger.EventHotKeyError slog record.
//
// Per F19 tolerance (architecture §Hotkey Message Pump Contract), a
// failed registration is non-fatal: the function returns the error to
// the caller, but main.go continues running with no active hotkey so
// the user can still trigger capture via the tray menu and inspect the
// log. Never panic on RegisterHotKey failure — a hotkey-in-use collision
// is a normal, expected condition (AC #7).
//
// On success, emits a logger.EventHotKeyRegistered slog.Info record with
// the "hotkey" field set to the original hotkeyStr argument — NEVER a
// hardcoded literal — so audit logs reflect the user's configured value
// (security-plan §Input Validation: log the validated input, not a
// pre-ordained string).
func Register(hwnd uintptr, hotkeyStr string, id int) error {
	mods, vk, err := ParseHotkeyString(hotkeyStr)
	if err != nil {
		lasterror.Set(err)
		slog.Error("hotkey parse failed",
			"event", logger.EventHotKeyError,
			"error", err.Error())
		return err
	}
	// RegisterHotKey signature (Win32): BOOL RegisterHotKey(HWND hWnd,
	// int id, UINT fsModifiers, UINT vk). Returns non-zero on success.
	ret, _, callErr := procRegisterHotKey.Call(
		hwnd,
		uintptr(id),
		uintptr(mods),
		uintptr(vk),
	)
	if ret == 0 {
		wrapped := fmt.Errorf("RegisterHotKey failed for %q: %w", hotkeyStr, callErr)
		lasterror.Set(wrapped)
		slog.Error("hotkey registration failed",
			"event", logger.EventHotKeyError,
			"error", wrapped.Error())
		return wrapped
	}
	slog.Info("hotkey registered",
		"event", logger.EventHotKeyRegistered,
		"hotkey", hotkeyStr)
	return nil
}
