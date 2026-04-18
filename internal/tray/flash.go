// Package tray — the 350ms safelight-amber capture flash (design-system
// signature element per Brand Identity). See Phase 3 plan Step 13a.
package tray

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/Turbolet85/clip-clap/internal/lasterror"
	"github.com/Turbolet85/clip-clap/internal/logger"
)

// Flash state. mu guards pendingRevert + the NIM_MODIFY swap path so
// concurrent callers cannot race on the timer or fire double-modifies.
var (
	flashMu       sync.Mutex
	pendingRevert *time.Timer
	hIconDefault  uintptr
	hIconAmber    uintptr
	iconsLoaded   bool
	iconsOnce     sync.Once
)

// EnsureIcons loads the tray's two ICO handles (default deep-ink at index 0,
// safelight-amber at index 1) from the running executable's embedded
// resource. Called lazily on first Flash invocation via a sync.Once guard
// so we avoid paying the ExtractIconW cost on startup. If the amber icon
// is missing from the build (e.g., assets/app-amber.ico wasn't compiled
// into resource.syso), the handle stays 0 and Flash() degrades gracefully
// via tray.flash.error logging.
func EnsureIcons() {
	iconsOnce.Do(func() {
		moduleHandle, _, _ := procGetModuleHandleW.Call(0)
		var exePath [windows.MAX_PATH]uint16
		n, _, _ := procGetModuleFileNameW.Call(
			moduleHandle,
			uintptr(unsafe.Pointer(&exePath[0])),
			uintptr(len(exePath)),
		)
		if n == 0 {
			return
		}

		// Index 0: default deep-ink aperture (the idle tray icon).
		h0, _, _ := procExtractIconW.Call(
			moduleHandle,
			uintptr(unsafe.Pointer(&exePath[0])),
			0,
		)
		if h0 != 0 && h0 != 1 {
			hIconDefault = h0
		}

		// Index 1: safelight-amber aperture (the flash-state icon). If
		// assets/app-amber.ico wasn't embedded, ExtractIconW returns 0.
		h1, _, _ := procExtractIconW.Call(
			moduleHandle,
			uintptr(unsafe.Pointer(&exePath[0])),
			1,
		)
		if h1 != 0 && h1 != 1 {
			hIconAmber = h1
		}

		iconsLoaded = true
	})
}

// Flash swaps the tray icon to the safelight-amber variant for exactly
// 350ms, then reverts to the default deep-ink. Re-entry handling: a second
// Flash() within 350ms stops the pending revert timer and restarts the
// countdown from `now` — no double-flash, no flicker, no intermediate
// state per design-system Brand Identity §Signature element.
//
// Failure mode: if the amber handle is unavailable (asset not embedded)
// OR Shell_NotifyIcon returns FALSE, emit `tray.flash.error` event via
// slog + surface the error to the tray via lasterror.Set. Do NOT retry,
// do NOT queue, do NOT crash — the tray remains in whatever state Win32
// left it (graceful degradation).
func Flash(hwnd uintptr) error {
	EnsureIcons()

	flashMu.Lock()
	defer flashMu.Unlock()

	if hIconAmber == 0 {
		err := fmt.Errorf("amber icon handle is nil (assets/app-amber.ico not embedded)")
		slog.Error("tray flash failed",
			"event", logger.EventTrayFlashError,
			"error", err.Error(),
		)
		lasterror.Set(SanitizeForTray(err))
		return err
	}

	if err := modifyTrayIcon(hwnd, hIconAmber); err != nil {
		slog.Error("tray flash failed",
			"event", logger.EventTrayFlashError,
			"error", err.Error(),
		)
		lasterror.Set(SanitizeForTray(err))
		return err
	}

	// Cancel any pending revert (restart countdown from now).
	if pendingRevert != nil {
		pendingRevert.Stop()
		pendingRevert = nil
	}
	pendingRevert = time.AfterFunc(350*time.Millisecond, func() {
		revertToDefault(hwnd)
	})
	return nil
}

// revertToDefault swaps the tray icon back to deep-ink. Called by the
// pending-revert time.AfterFunc exactly 350ms after the last Flash.
// Ignores Shell_NotifyIcon failure (tray stays in whatever state Win32
// left it — never retry).
func revertToDefault(hwnd uintptr) {
	flashMu.Lock()
	defer flashMu.Unlock()
	pendingRevert = nil
	if hIconDefault == 0 {
		return
	}
	_ = modifyTrayIcon(hwnd, hIconDefault)
}

// modifyTrayIcon issues Shell_NotifyIconW(NIM_MODIFY) to swap the HIcon on
// the already-registered tray entry. Returns an error on FALSE return.
func modifyTrayIcon(hwnd, hIcon uintptr) error {
	iconState.Lock()
	uid := iconState.uid
	iconState.Unlock()
	nid := NOTIFYICONDATAW{
		CbSize: notifyIconStructSize(),
		HWnd:   hwnd,
		UID:    uid,
		UFlags: NIF_ICON,
		HIcon:  hIcon,
	}
	ret, _, callErr := procShellNotifyIconW.Call(
		uintptr(NIM_MODIFY),
		uintptr(unsafe.Pointer(&nid)),
	)
	if ret == 0 {
		return fmt.Errorf("Shell_NotifyIconW(NIM_MODIFY): %w", callErr)
	}
	return nil
}

// UpdateTooltipAfterCapture swaps the tray tooltip to "Last: <filename>"
// (basename only) after a successful capture. Uses Shell_NotifyIcon(NIM_MODIFY)
// with NIF_TIP to push just the tooltip field.
func UpdateTooltipAfterCapture(hwnd uintptr, filename string) error {
	nid := NOTIFYICONDATAW{
		CbSize: notifyIconStructSize(),
		HWnd:   hwnd,
		UID:    trayUID,
		UFlags: NIF_TIP,
	}
	tooltip := fmt.Sprintf("Last: %s", filename)
	copyTooltipToSzTip(&nid.SzTip, tooltip)
	ret, _, callErr := procShellNotifyIconW.Call(
		uintptr(NIM_MODIFY),
		uintptr(unsafe.Pointer(&nid)),
	)
	if ret == 0 {
		return fmt.Errorf("Shell_NotifyIconW(NIM_MODIFY tooltip): %w", callErr)
	}
	return nil
}

// RevertTooltip restores the idle "clip-clap" tooltip (post-Undo).
func RevertTooltip(hwnd uintptr) error {
	nid := NOTIFYICONDATAW{
		CbSize: notifyIconStructSize(),
		HWnd:   hwnd,
		UID:    trayUID,
		UFlags: NIF_TIP,
	}
	copyTooltipToSzTip(&nid.SzTip, BuildTooltip())
	ret, _, callErr := procShellNotifyIconW.Call(
		uintptr(NIM_MODIFY),
		uintptr(unsafe.Pointer(&nid)),
	)
	if ret == 0 {
		return fmt.Errorf("Shell_NotifyIconW(NIM_MODIFY tooltip revert): %w", callErr)
	}
	return nil
}
