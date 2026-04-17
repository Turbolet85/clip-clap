package hotkey

import (
	"testing"
)

// TestParseHotkeyString_CtrlShiftS pins the default hotkey's parse
// output. "Ctrl+Shift+S" is the architecture-specified default and AC #5
// asserts on its literal appearance in the log — the parser is the only
// path that maps the config string to the Win32 (mods, vk) pair, so
// this test is the contract's single guarantor.
func TestParseHotkeyString_CtrlShiftS(t *testing.T) {
	mods, vk, err := ParseHotkeyString("Ctrl+Shift+S")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantMods := MOD_CONTROL | MOD_SHIFT
	if mods != wantMods {
		t.Errorf("mods: want 0x%x, got 0x%x", wantMods, mods)
	}
	// 'S' letter key uses its ASCII uppercase value as the VK code.
	if vk != uint32('S') {
		t.Errorf("vk: want 0x%x, got 0x%x", 'S', vk)
	}
}

// TestParseHotkeyString_TableDriven exercises the three canonical
// modifier-count / named-key / Win-modifier shapes so regression on any
// one token variant surfaces from a single test failure.
func TestParseHotkeyString_TableDriven(t *testing.T) {
	cases := []struct {
		in       string
		wantMods uint32
		wantVK   uint32
	}{
		{"Alt+F1", MOD_ALT, VK_F1},
		{"Ctrl+Alt+Shift+Space", MOD_CONTROL | MOD_ALT | MOD_SHIFT, VK_SPACE},
		{"Win+V", MOD_WIN, uint32('V')},
		// Additional variants that the Security Pattern-A map-based
		// parser should handle correctly:
		{"Ctrl+PageDown", MOD_CONTROL, VK_NEXT},
		{"Shift+F12", MOD_SHIFT, VK_F12},
		{"Alt+0", MOD_ALT, uint32('0')},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			mods, vk, err := ParseHotkeyString(c.in)
			if err != nil {
				t.Fatalf("%q: unexpected error: %v", c.in, err)
			}
			if mods != c.wantMods {
				t.Errorf("%q mods: want 0x%x, got 0x%x", c.in, c.wantMods, mods)
			}
			if vk != c.wantVK {
				t.Errorf("%q vk: want 0x%x, got 0x%x", c.in, c.wantVK, vk)
			}
		})
	}
}

// TestParseHotkeyString_InvalidRejected is the allowlist-enforcement
// gate required by security-plan §Input Validation. Every invalid shape
// listed in the task-tests.md spec + case-variation rejection surfaces
// a non-nil error. If this test ever passes a case-normalized input
// like "CTRL+S", the allowlist has been bypassed — the contract is
// broken and a security review is required before release.
func TestParseHotkeyString_InvalidRejected(t *testing.T) {
	invalids := []string{
		"",             // empty
		"Ctrl+",        // dangling separator / no key
		"+S",           // empty modifier token
		"Nope+S",       // unknown modifier
		"Ctrl+NotAKey", // unknown key
		"CTRL+S",       // case variation — MUST be rejected (not ToUpper'd)
		"ctrl+s",       // lowercase variation
		"S",            // no modifier pair
		"Ctrl++S",      // consecutive separators → empty middle token
	}
	for _, in := range invalids {
		t.Run(in, func(t *testing.T) {
			_, _, err := ParseHotkeyString(in)
			if err == nil {
				t.Errorf("%q: expected non-nil error, got nil (allowlist bypass)", in)
			}
		})
	}
}
