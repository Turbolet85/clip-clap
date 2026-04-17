package tray

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// Win32 procs consumed by tray.go and ShowContextMenu. golang.org/x/sys/
// windows does not export Shell_NotifyIconW, TrackPopupMenuEx, CreateMenu,
// InsertMenuItemW, DestroyMenu, SetForegroundWindow, GetCursorPos, or
// AppendMenuW. Lazy-load them from the relevant DLLs. Lazy loading defers
// the DLL resolution until first use — acceptable here since every call
// site is behind the tray message-pump gate (only runs when the UI thread
// is alive).
//
// DLL attribution:
//   - user32.dll — GetCursorPos, SetForegroundWindow, CreatePopupMenu,
//     AppendMenuW, TrackPopupMenuEx, DestroyMenu, DefWindowProcW,
//     LoadIconW, PostMessageW, RegisterClassExW, UnregisterClassW,
//     CreateWindowExW, DestroyWindow, GetMessageW, TranslateMessage,
//     DispatchMessageW
//   - shell32.dll — Shell_NotifyIconW
//   - kernel32.dll — GetModuleHandleW (for LoadIcon module handle)
var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	shell32  = windows.NewLazySystemDLL("shell32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procShellNotifyIconW    = shell32.NewProc("Shell_NotifyIconW")
	procExtractIconW        = shell32.NewProc("ExtractIconW")
	procGetModuleHandleW    = kernel32.NewProc("GetModuleHandleW")
	procGetModuleFileNameW  = kernel32.NewProc("GetModuleFileNameW")
	procLoadIconW           = user32.NewProc("LoadIconW")
	procCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	procAppendMenuW         = user32.NewProc("AppendMenuW")
	procTrackPopupMenuEx    = user32.NewProc("TrackPopupMenuEx")
	procDestroyMenu         = user32.NewProc("DestroyMenu")
	procGetCursorPos        = user32.NewProc("GetCursorPos")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procPostMessageW        = user32.NewProc("PostMessageW")
)

// NOTIFYICONDATAW is the Shell_NotifyIcon payload. Field order must match
// the Win32 layout byte-for-byte; reordering breaks the silent-field-drop
// behavior where Shell_NotifyIconW ignores fields past CbSize. We use only
// the fields required by Phase 2 (icon + tooltip + callback) and omit the
// rest (BalloonTitle, BalloonText, InfoFlags, etc.) — they're zeroed.
// Total size matches NOTIFYICONDATAW_V3_SIZE on Win10+; CbSize MUST be
// initialized from unsafe.Sizeof at every call site (Step 6 enforces).
type NOTIFYICONDATAW struct {
	CbSize           uint32
	HWnd             uintptr
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            uintptr
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UVersion         uint32 // union with UTimeout
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         windows.GUID
	HBalloonIcon     uintptr
}

// Shell_NotifyIcon dwMessage values.
const (
	NIM_ADD    = 0x0
	NIM_MODIFY = 0x1
	NIM_DELETE = 0x2
)

// Shell_NotifyIcon uFlags bitmask — which struct fields are honored.
const (
	NIF_MESSAGE = 0x1
	NIF_ICON    = 0x2
	NIF_TIP     = 0x4
)

// Win32 window-message constants consumed by WndProc dispatch. The full
// list is in Win32 WM_* reference; we expose only what Phase 2's tray
// surface touches — the overlay/message-pump Phase 3+ additions will
// extend this block. golang.org/x/sys/windows v0.24.0 does not export
// the WM_* namespace, so we define values locally.
const (
	WM_APP       = 0x8000
	WM_USER      = 0x0400
	WM_CLOSE     = 0x0010
	WM_QUIT      = 0x0012
	WM_COMMAND   = 0x0111
	WM_HOTKEY    = 0x0312
	WM_RBUTTONUP = 0x0205
	WM_LBUTTONUP = 0x0202
	TrayCallback = WM_USER + 1 // NOTIFYICONDATAW.UCallbackMessage — unique-enough that tray dispatch can route it
)

// TrackPopupMenuEx flag bits. TPM_RETURNCMD makes the call synchronous
// (returns the chosen menu ID), but we use fire-and-forget dispatch via
// WM_COMMAND, so omit it — the default behavior posts WM_COMMAND to hwnd
// and returns immediately.
const (
	TPM_RIGHTBUTTON = 0x0002
	TPM_BOTTOMALIGN = 0x0020
)

// AppendMenuW flags.
const (
	MF_STRING   = 0x0000
	MF_GRAYED   = 0x0001
	MF_DISABLED = 0x0002
)

// POINT is the layout of WinAPI POINT; consumed by GetCursorPos in
// ShowContextMenu to anchor the menu at the cursor.
type POINT struct {
	X int32
	Y int32
}

// notifyIconStructSize returns the Win32 cbSize value used for NIM_ADD /
// NIM_MODIFY / NIM_DELETE calls. Centralized so RegisterIcon, Unregister,
// and any future modifier all use the same value without repeating the
// unsafe.Sizeof incantation.
func notifyIconStructSize() uint32 {
	return uint32(unsafe.Sizeof(NOTIFYICONDATAW{}))
}

// GetModuleHandle is a thin wrapper around kernel32's GetModuleHandleW with
// a nil lpModuleName (returns the handle for the current process's EXE).
// Exported so cmd/clip-clap/main.go can pass the hInstance into
// RegisterClassExW and CreateWindowExW without re-loading kernel32.
// Returns (handle, lastError-ret, call-err) matching the raw LazyProc.Call
// convention so the caller can decide which of the three return slots
// carries the diagnostic it cares about.
func GetModuleHandle() (uintptr, uintptr, error) {
	return procGetModuleHandleW.Call(0)
}
