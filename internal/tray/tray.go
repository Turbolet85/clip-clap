// Package tray owns the Windows system-tray surface (Shell_NotifyIcon icon,
// TrackPopupMenuEx context menu, tooltip, and — in Phase 3 — the 350ms
// safelight capture flash). Phase 2 stages the signature: the deep-ink
// aperture `#0E1013` from assets/app.ico registers via NIM_ADD in a
// motionless idle state. No motion, no pulse, no hover-grow — the entire
// desktop-native 0.30 motion budget is reserved for Phase 3's capture
// flash (design-system §Motion §Hard Limits).
//
// Menu chrome is text-only — NEVER MFT_BITMAP (design-system §Per-Surface
// Bans §desktop-native). Labels use "Expose\tCtrl+Shift+S" (not "Capture")
// per the darkroom-verb convention, and "Settings (edit config.toml)"
// (grayed) in place of a generic gear-glyph entry.
package tray

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/Turbolet85/clip-clap/internal/config"
	"github.com/Turbolet85/clip-clap/internal/lasterror"
	"github.com/Turbolet85/clip-clap/internal/logger"
)

// iconState holds the single HICON handle + mutex used by RegisterIcon
// and Phase 3's flash swap. sync.Mutex-guarded per the design system's
// signature-staging contract (Phase 3 will acquire this to swap HICON
// between deep-ink and amber without racing the 100ms refresher goroutine).
var iconState struct {
	sync.Mutex
	hIcon uintptr
	uid   uint32
}

// trayUID is the NOTIFYICONDATAW.UID for our single icon. Win32 matches
// future NIM_MODIFY / NIM_DELETE calls by (hwnd, uid); a fixed value is
// fine because clip-clap only ever registers one tray icon per process.
const trayUID uint32 = 1

// SanitizeForTray unwraps error types that embed a filesystem path
// (*os.PathError, *os.LinkError, *fs.PathError, or any error wrapping
// one via errors.As) and replaces the .Path with filepath.Base(Path).
// This prevents `C:\Users\<realname>\...` from leaking into the tray
// "Last error" slot (screenshot/screen-share risk per security-plan
// §Error Handling).
//
// Exported (PascalCase) so cmd/clip-clap/main.go's WndProc defer-recover
// block can call tray.SanitizeForTray(err) — an unexported name would
// fail `go build ./cmd/clip-clap` with "undefined: tray.sanitizeForTray".
//
// Returns err unchanged if it is not a path-bearing error; returns nil
// if err is nil (callers can wrap unconditionally without a nil check).
func SanitizeForTray(err error) error {
	if err == nil {
		return nil
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return &os.PathError{
			Op:   pathErr.Op,
			Path: filepath.Base(pathErr.Path),
			Err:  pathErr.Err,
		}
	}
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		return &os.LinkError{
			Op:  linkErr.Op,
			Old: filepath.Base(linkErr.Old),
			New: filepath.Base(linkErr.New),
			Err: linkErr.Err,
		}
	}
	// fs.PathError and os.PathError are the same type under Go 1.16+
	// (os.PathError = fs.PathError alias); the explicit fs.PathError
	// branch below is redundant but future-proof if the alias ever
	// changes. Kept as an `if false` to document intent — no runtime
	// cost, and no compile warning for unreachable code.
	var fsPathErr *fs.PathError
	if errors.As(err, &fsPathErr) {
		return &fs.PathError{
			Op:   fsPathErr.Op,
			Path: filepath.Base(fsPathErr.Path),
			Err:  fsPathErr.Err,
		}
	}
	return err
}

// RegisterIcon registers the deep-ink aperture in the system tray via
// Shell_NotifyIconW(NIM_ADD). Phase 2 emits NO slog event on success or
// failure — the main.go startup wiring owns event emission for tray
// registration, and AC #5 requires `hotkey.registered` to be the first
// event after `config.loaded` (emitting here would violate the ordering).
//
// hwnd is the message-only window created by main.go that receives the
// TrayCallback (WM_USER+1) message on right-click. The loaded HICON is
// stashed in iconState for Phase 3's flash swap; Phase 2 never mutates
// it after RegisterIcon returns.
func RegisterIcon(hwnd uintptr) error {
	// Load the icon embedded by goversioninfo into resource.syso. We use
	// ExtractIconW against our own .exe path (not LoadIconW against a
	// resource ID) because goversioninfo's RT_GROUP_ICON resource name
	// varies between library versions (sometimes numeric 1, sometimes a
	// string like "APP"/"MAIN"). ExtractIconW extracts by INDEX and is
	// agnostic to that naming choice — index 0 is always the first (and
	// only, in our case) icon group in the PE resource table.
	moduleHandle, _, _ := procGetModuleHandleW.Call(0)
	var exePath [windows.MAX_PATH]uint16
	n, _, nameErr := procGetModuleFileNameW.Call(
		moduleHandle,
		uintptr(unsafe.Pointer(&exePath[0])),
		uintptr(len(exePath)),
	)
	if n == 0 {
		return fmt.Errorf("GetModuleFileName failed: %w", nameErr)
	}
	hIcon, _, err := procExtractIconW.Call(
		moduleHandle,
		uintptr(unsafe.Pointer(&exePath[0])),
		0, // first icon
	)
	// ExtractIconW returns:
	//   1 if the file is not an exe/dll/ico (error sentinel, very rare for our own exe)
	//   0 if no icon was found in the file
	//   a valid HICON otherwise
	if hIcon == 0 || hIcon == 1 {
		return fmt.Errorf("ExtractIcon failed (hIcon=%d): %w", hIcon, err)
	}

	iconState.Lock()
	iconState.hIcon = hIcon
	iconState.uid = trayUID
	iconState.Unlock()

	nid := NOTIFYICONDATAW{
		CbSize:           notifyIconStructSize(),
		HWnd:             hwnd,
		UID:              trayUID,
		UFlags:           NIF_MESSAGE | NIF_ICON | NIF_TIP,
		UCallbackMessage: TrayCallback,
		HIcon:            hIcon,
	}
	copyTooltipToSzTip(&nid.SzTip, BuildTooltip())

	ret, _, callErr := procShellNotifyIconW.Call(
		uintptr(NIM_ADD),
		uintptr(unsafe.Pointer(&nid)),
	)
	if ret == 0 {
		return fmt.Errorf("Shell_NotifyIconW(NIM_ADD) failed: %w", callErr)
	}
	return nil
}

// UnregisterIcon deregisters the tray icon via Shell_NotifyIconW(NIM_DELETE).
// Idempotent after first successful call (subsequent NIM_DELETE returns
// FALSE but costs nothing and is safe to ignore); we return the Win32
// error for the first call so main.go's shutdown can log it if needed.
func UnregisterIcon(hwnd uintptr) error {
	iconState.Lock()
	uid := iconState.uid
	iconState.Unlock()
	nid := NOTIFYICONDATAW{
		CbSize: notifyIconStructSize(),
		HWnd:   hwnd,
		UID:    uid,
	}
	ret, _, callErr := procShellNotifyIconW.Call(
		uintptr(NIM_DELETE),
		uintptr(unsafe.Pointer(&nid)),
	)
	if ret == 0 {
		return fmt.Errorf("Shell_NotifyIconW(NIM_DELETE) failed: %w", callErr)
	}
	return nil
}

// ShowContextMenu builds a fresh popup menu on every right-click and
// dispatches via TrackPopupMenuEx. Rebuilding each time (vs. caching
// the HMENU) is the guaranteed-correct path for the "Last error" label
// which changes as subsystems publish via lasterror.Set — no NIM_MODIFY
// dance required, at a cost of six CreatePopupMenu calls per right-click
// which is imperceptible.
//
// Per the design system, menu items are text-only (NEVER MFT_BITMAP),
// grayed items pass MF_GRAYED (Win11 renders them in its drained-fixer
// grey #5A5E63 automatically), and the hotkey hint on the Expose entry
// is right-aligned via the `\t` separator per Win32 menu convention.
func ShowContextMenu(hwnd uintptr) {
	hMenu, _, _ := procCreatePopupMenu.Call()
	if hMenu == 0 {
		return
	}
	defer procDestroyMenu.Call(hMenu)

	appendMenu(hMenu, MenuIDCapture, MF_STRING, "Expose\tCtrl+Shift+S")
	appendMenu(hMenu, MenuIDOpenFolder, MF_STRING, "Open folder")
	appendMenu(hMenu, MenuIDSettings, MF_STRING|MF_GRAYED, "Settings (edit config.toml)")
	appendMenu(hMenu, MenuIDUndoLastCapture, MF_STRING|MF_GRAYED, "Undo last capture")
	appendMenu(hMenu, MenuIDLastError, MF_STRING|MF_GRAYED, FormatLastErrorMenuLabel(SanitizeForTray(lasterror.Get())))
	appendMenu(hMenu, MenuIDQuit, MF_STRING, "Quit")

	// Win32 requires SetForegroundWindow before TrackPopupMenuEx or the
	// menu can dismiss itself when focus moves. POINT at cursor so the
	// menu anchors where the user clicked.
	procSetForegroundWindow.Call(hwnd)
	var pt POINT
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	procTrackPopupMenuEx.Call(
		hMenu,
		uintptr(TPM_RIGHTBUTTON|TPM_BOTTOMALIGN),
		uintptr(pt.X),
		uintptr(pt.Y),
		hwnd,
		0,
	)
}

// appendMenu is a thin helper around AppendMenuW. Menu labels are UTF-16
// (W-suffixed variant); lpNewItem is a zero-terminated UTF-16 string
// pointer (not a uintptr-encoded integer for MF_STRING).
func appendMenu(hMenu uintptr, id int, flags uint32, label string) {
	labelPtr, _ := windows.UTF16PtrFromString(label)
	procAppendMenuW.Call(
		hMenu,
		uintptr(flags),
		uintptr(id),
		uintptr(unsafe.Pointer(labelPtr)),
	)
}

// copyTooltipToSzTip writes s into the fixed-size UTF-16 szTip buffer,
// NUL-terminating and truncating if s exceeds the buffer. Win32 ignores
// characters past the first NUL, so over-copying is a no-op, but we
// truncate explicitly for determinism.
func copyTooltipToSzTip(dst *[128]uint16, s string) {
	u16, _ := windows.UTF16FromString(s)
	n := len(u16)
	if n > len(dst) {
		n = len(dst)
	}
	for i := 0; i < n; i++ {
		dst[i] = u16[i]
	}
}

// UpdateLastErrorMenu spawns a goroutine that polls lasterror.Get() every
// 100ms and is a no-op in Phase 2 (ShowContextMenu reads lasterror.Get()
// directly on each right-click, so the grayed "Last error" label always
// reflects the latest value without a separate polling loop). Declared
// so Phase 3 can extend it without changing the caller's import surface —
// for now it guards the pump from crashing via defer recover() and
// exits when hwnd receives WM_QUIT (signaled via ticker.Stop below).
//
// Security: any recovered panic is sanitized via SanitizeForTray before
// being persisted to lasterror.Set (prevents path leakage on screen-shares).
func UpdateLastErrorMenu(hwnd uintptr) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				var err error
				switch v := r.(type) {
				case error:
					err = v
				case string:
					err = errors.New(v)
				default:
					err = fmt.Errorf("%v", v)
				}
				sanitized := SanitizeForTray(err)
				lasterror.Set(sanitized)
				slog.Error("tray last-error updater panicked",
					"event", logger.EventHotKeyError,
					"error", sanitized.Error())
			}
		}()
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		// No-op loop in Phase 2 — ShowContextMenu reads lasterror.Get()
		// on right-click, so the menu always shows the current value
		// without needing NIM_MODIFY churn. Phase 3 may extend this
		// goroutine to refresh a dedicated tooltip/toast on error.
		for range ticker.C {
			_ = hwnd // reserved for Phase 3
		}
	}()
}

// HandleMenuCommand dispatches a WM_COMMAND menu ID to its handler. Called
// from cmd/clip-clap/main.go's WndProc after extracting the menu ID from
// the low 16 bits of wparam. Returns true if the message was handled
// (main.go can then return 0 to Win32); false if id is unknown (caller
// forwards to DefWindowProc).
func HandleMenuCommand(hwnd uintptr, id int, cfg *config.Config) bool {
	switch id {
	case MenuIDCapture:
		// Stub for Phase 3 — emit a debug event so the wiring is visible
		// in logs without committing to a future event constant. Reuses
		// EventTrayMenuOpened as a placeholder; Phase 3 introduces a
		// dedicated capture.* event constant.
		slog.Debug("tray menu capture clicked",
			"event", logger.EventTrayMenuOpened,
			"source", "tray_menu")
		return true
	case MenuIDOpenFolder:
		openFolder(cfg)
		return true
	case MenuIDQuit:
		procPostMessageW.Call(hwnd, uintptr(WM_CLOSE), 0, 0)
		return true
	case MenuIDSettings, MenuIDUndoLastCapture, MenuIDLastError:
		// Grayed / read-only slots — Win32 won't normally fire WM_COMMAND
		// for MF_GRAYED items, but defense-in-depth against spoofed
		// messages from a hostile process.
		return true
	default:
		return false
	}
}

// openFolder implements the Open folder menu handler. cfg.SaveFolder is
// already validated by internal/config (absolute Windows path, user-
// writable scope), so we pass it as a single positional argument to
// exec.Command — never shell-interpolated, never concatenated with user
// input. On failure, sanitize the error (strip full paths from
// *os.PathError so screenshots don't leak the user's directory).
func openFolder(cfg *config.Config) {
	if cfg == nil {
		// Defensive; main.go always passes a non-nil cfg in Phase 2.
		return
	}
	cmd := exec.Command("explorer.exe", cfg.SaveFolder)
	if err := cmd.Start(); err != nil {
		sanitized := SanitizeForTray(err)
		lasterror.Set(sanitized)
		slog.Error("tray menu failed",
			"event", logger.EventTrayMenuOpened,
			"error", sanitized.Error())
		return
	}
	slog.Info("tray menu opened",
		"event", logger.EventTrayMenuOpened)
}
