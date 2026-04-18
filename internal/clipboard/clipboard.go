// Package clipboard owns Win32 OpenClipboard / SetClipboardData(CF_UNICODETEXT)
// writes plus the in-memory Undo snapshot and the per-capture 500ms reentry
// guard. See Phase 3 plan Step 9 and architecture.md §[Clipboard Write] for
// the canonical flow.
package clipboard

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
	"unsafe"

	"github.com/Turbolet85/clip-clap/internal/logger"
)

// Package state — all protected by `mu`.
var (
	mu sync.Mutex

	// lastSnapshot holds the prior clipboard contents (UTF-16 encoded) at
	// the time of the most recent successful Swap call. A nil snapshot
	// means there's nothing to undo. This is NEVER logged (privacy: prior
	// clipboard may contain passwords or PII per security-plan).
	lastSnapshot []uint16

	// reentryGuard tracks per-capture-id active guards. Set on Swap,
	// cleared via afterFunc 500ms later.
	reentryGuard sync.Map

	// afterFunc is injectable for deterministic testing. Defaults to
	// time.AfterFunc; tests can substitute a stub that records calls.
	afterFunc = func(d time.Duration, f func()) *time.Timer {
		return time.AfterFunc(d, f)
	}
)

// Swap writes absPath to the clipboard as CF_UNICODETEXT, saving the prior
// contents into the Undo snapshot. Emits `clipboard.swap` event on success.
// Installs a 500ms per-captureID reentry guard so a second Swap within
// 500ms using the same captureID is silently dropped (prevents clipboard
// re-read loops when the system's own change-notification fires back).
//
// On error at any stage (OpenClipboard, GlobalAlloc, etc.), returns the
// error WITHOUT emitting clipboard.swap. The caller (main.go captureCallback)
// is responsible for emitting capture.failed and sanitizing the error
// before calling lasterror.Set.
func Swap(absPath, captureID string) error {
	if IsGuarded(captureID) {
		// Silent drop per security-plan Reentry Guard.
		return nil
	}

	// Open clipboard (hwnd=0 means associate with current thread, which
	// is sufficient for a single-threaded Win32 message pump).
	ret, _, err := procOpenClipboard.Call(0)
	if ret == 0 {
		return fmt.Errorf("OpenClipboard: %w", err)
	}
	defer procCloseClipboard.Call()

	// Snapshot prior clipboard text BEFORE emptying.
	snapshotPriorClipboard()

	// Empty + allocate the new payload.
	procEmptyClipboard.Call()

	payload, err := utf16FromString(absPath)
	if err != nil {
		return fmt.Errorf("encode utf16: %w", err)
	}
	bytesNeeded := (len(payload) + 1) * 2 // UTF-16 code units + NUL terminator

	hGlobal, _, gAllocErr := procGlobalAlloc.Call(uintptr(GMEM_MOVEABLE|GMEM_ZEROINIT), uintptr(bytesNeeded))
	if hGlobal == 0 {
		return fmt.Errorf("GlobalAlloc: %w", gAllocErr)
	}

	// On any error between here and SetClipboardData success, free hGlobal.
	ownershipTransferred := false
	defer func() {
		if !ownershipTransferred {
			procGlobalFree.Call(hGlobal)
		}
	}()

	dst, _, lockErr := procGlobalLock.Call(hGlobal)
	if dst == 0 {
		return fmt.Errorf("GlobalLock: %w", lockErr)
	}
	// Copy UTF-16 units + NUL terminator into the HGLOBAL via RtlMoveMemory.
	// Using Win32 memcpy (RtlMoveMemory) avoids the `unsafe.Pointer(uintptr)`
	// pattern that go vet's unsafeptr check flags — `uintptr(unsafe.Pointer(&payload[0]))`
	// is the vet-approved direction because the payload slice is Go-managed.
	withTerm := append(payload, 0) // NOTE: copy, not in-place alias
	procRtlMoveMemory.Call(
		dst,
		uintptr(unsafe.Pointer(&withTerm[0])),
		uintptr(len(withTerm)*2),
	)
	procGlobalUnlock.Call(hGlobal)

	// Hand ownership to the clipboard. On success, the clipboard frees
	// the HGLOBAL — we MUST NOT free it again.
	setRet, _, setErr := procSetClipboardData.Call(uintptr(CF_UNICODETEXT), hGlobal)
	if setRet == 0 {
		return fmt.Errorf("SetClipboardData: %w", setErr)
	}
	ownershipTransferred = true

	installReentryGuard(captureID)

	slog.Info("clipboard swapped",
		"event", logger.EventClipboardSwap,
		"capture_id", captureID,
		"path", absPath,
	)
	return nil
}

// Undo restores the prior clipboard contents (captured by the most recent
// Swap) and clears the snapshot. No-op if no snapshot exists. Emits
// `clipboard.undo` event with NO path/error fields (privacy per
// security-plan).
func Undo() error {
	mu.Lock()
	snapshot := lastSnapshot
	mu.Unlock()
	if snapshot == nil {
		return nil
	}

	ret, _, err := procOpenClipboard.Call(0)
	if ret == 0 {
		return fmt.Errorf("OpenClipboard: %w", err)
	}
	defer procCloseClipboard.Call()

	procEmptyClipboard.Call()

	bytesNeeded := (len(snapshot) + 1) * 2
	hGlobal, _, gAllocErr := procGlobalAlloc.Call(uintptr(GMEM_MOVEABLE|GMEM_ZEROINIT), uintptr(bytesNeeded))
	if hGlobal == 0 {
		return fmt.Errorf("GlobalAlloc: %w", gAllocErr)
	}

	ownershipTransferred := false
	defer func() {
		if !ownershipTransferred {
			procGlobalFree.Call(hGlobal)
		}
	}()

	dst, _, lockErr := procGlobalLock.Call(hGlobal)
	if dst == 0 {
		return fmt.Errorf("GlobalLock: %w", lockErr)
	}
	withTerm := append([]uint16(nil), snapshot...)
	withTerm = append(withTerm, 0)
	procRtlMoveMemory.Call(
		dst,
		uintptr(unsafe.Pointer(&withTerm[0])),
		uintptr(len(withTerm)*2),
	)
	procGlobalUnlock.Call(hGlobal)

	setRet, _, setErr := procSetClipboardData.Call(uintptr(CF_UNICODETEXT), hGlobal)
	if setRet == 0 {
		return fmt.Errorf("SetClipboardData: %w", setErr)
	}
	ownershipTransferred = true

	mu.Lock()
	lastSnapshot = nil
	mu.Unlock()

	slog.Info("clipboard undo",
		"event", logger.EventClipboardUndo,
	)
	return nil
}

// HasSnapshot returns true when a prior clipboard snapshot exists and Undo
// can restore it. Used by tray menu to enable/disable the Undo entry.
func HasSnapshot() bool {
	mu.Lock()
	defer mu.Unlock()
	return lastSnapshot != nil
}

// SetGuard (testing) installs a reentry guard for captureID without the
// 500ms timer — used by unit tests to verify guard lookup.
func SetGuard(captureID string) {
	reentryGuard.Store(captureID, struct{}{})
}

// ClearGuard (testing) removes a reentry guard for captureID.
func ClearGuard(captureID string) {
	reentryGuard.Delete(captureID)
}

// IsGuarded returns true when a reentry guard is currently active for the
// given captureID. Called silently — no log emission per security-plan.
func IsGuarded(captureID string) bool {
	_, ok := reentryGuard.Load(captureID)
	return ok
}

// SetAfterFunc (testing) substitutes the time.AfterFunc used to clear
// reentry guards after 500ms. Tests call this with a stub that records the
// pending callback, then manually triggers it to simulate time advancement.
func SetAfterFunc(f func(time.Duration, func()) *time.Timer) {
	afterFunc = f
}

// ResetAfterFunc (testing) restores the production afterFunc (time.AfterFunc).
func ResetAfterFunc() {
	afterFunc = func(d time.Duration, f func()) *time.Timer {
		return time.AfterFunc(d, f)
	}
}

// installReentryGuard sets the guard flag for captureID and schedules an
// auto-clear after 500ms via afterFunc (injectable for testing).
func installReentryGuard(captureID string) {
	reentryGuard.Store(captureID, struct{}{})
	afterFunc(500*time.Millisecond, func() {
		reentryGuard.Delete(captureID)
	})
}

// snapshotPriorClipboard reads the current CF_UNICODETEXT payload (if any)
// into lastSnapshot, under the mutex. Called with the clipboard already
// open (no OpenClipboard/CloseClipboard pair inside).
func snapshotPriorClipboard() {
	handle, _, _ := procGetClipboardData.Call(uintptr(CF_UNICODETEXT))
	if handle == 0 {
		mu.Lock()
		lastSnapshot = nil
		mu.Unlock()
		return
	}
	locked, _, _ := procGlobalLock.Call(handle)
	if locked == 0 {
		return
	}
	defer procGlobalUnlock.Call(handle)
	sizeRet, _, _ := procGlobalSize.Call(handle)
	if sizeRet == 0 {
		return
	}
	// Size is in bytes; UTF-16 char is 2 bytes.
	n := int(sizeRet) / 2
	if n <= 0 {
		return
	}
	// Read from HGLOBAL via RtlMoveMemory (Win32 memcpy). Using the
	// Win32 copy call sidesteps go vet's unsafeptr check, which flags
	// `unsafe.Pointer(uintptr)` conversions (HGLOBAL memory is Win32-
	// managed, not GC-movable, but vet cannot tell).
	snap := make([]uint16, n)
	procRtlMoveMemory.Call(
		uintptr(unsafe.Pointer(&snap[0])),
		locked,
		uintptr(n*2),
	)
	// Trim trailing zeros (NUL terminator).
	for len(snap) > 0 && snap[len(snap)-1] == 0 {
		snap = snap[:len(snap)-1]
	}
	mu.Lock()
	lastSnapshot = snap
	mu.Unlock()
}

// SetLastSnapshotForTesting overrides lastSnapshot for unit tests.
func SetLastSnapshotForTesting(s []uint16) {
	mu.Lock()
	lastSnapshot = s
	mu.Unlock()
}

// GetLastSnapshotForTesting returns the current snapshot (or nil) for tests.
func GetLastSnapshotForTesting() []uint16 {
	mu.Lock()
	defer mu.Unlock()
	return lastSnapshot
}

// utf16FromString converts a Go string to a UTF-16 slice (NO trailing NUL).
// The NUL is added separately when copying into HGLOBAL.
func utf16FromString(s string) ([]uint16, error) {
	if s == "" {
		return nil, errors.New("empty string")
	}
	runes := []rune(s)
	out := make([]uint16, 0, len(runes))
	for _, r := range runes {
		if r < 0x10000 {
			out = append(out, uint16(r))
		} else {
			// Surrogate pair encoding.
			r -= 0x10000
			out = append(out, 0xD800+uint16(r>>10), 0xDC00+uint16(r&0x3FF))
		}
	}
	return out, nil
}
