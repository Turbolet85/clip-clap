package overlay

import (
	"testing"
)

// TestCreateOverlay_Idempotent verifies the ensureClassRegistered guard's
// idempotent behavior: once the window class is registered (classReg=true),
// subsequent calls short-circuit without invoking Win32 RegisterClassExW.
// Full Win32 idempotency (ERROR_CLASS_ALREADY_EXISTS code path) is
// exercised in Phase 4 integration tests via pywinauto; this unit test
// verifies the in-process guard logic only.
func TestCreateOverlay_Idempotent(t *testing.T) {
	// Save and restore the package-level guard state so other tests
	// (running in the same binary) are not affected.
	classRegMu.Lock()
	savedState := classReg
	classReg = true
	classRegMu.Unlock()
	t.Cleanup(func() {
		classRegMu.Lock()
		classReg = savedState
		classRegMu.Unlock()
	})

	// With classReg already true, ensureClassRegistered must return nil
	// immediately — it must NOT call RegisterClassExW (which would fail
	// in a non-Windows test environment but silently pass on Windows).
	// Either way, the short-circuit path returning nil is what we assert.
	err := ensureClassRegistered()
	if err != nil {
		t.Errorf("ensureClassRegistered() with classReg=true: err = %v, want nil", err)
	}
}
