package main

import (
	"golang.org/x/sys/windows"
)

// Win32 procs for the message-only window and message pump. These are
// distinct from the tray-icon Win32 surface (internal/tray/win32.go) —
// keep the separation so main.go owns the pump lifecycle and the tray
// package owns the icon/menu surface.
//
// golang.org/x/sys/windows v0.24.0 exposes minimal Win32 wrappers; none
// of RegisterClassExW, CreateWindowExW, DefWindowProcW, the GetMessageW/
// TranslateMessage/DispatchMessageW pump primitives, DestroyWindow, or
// UnregisterClassW are pre-wrapped. Resolve them via lazy DLL load.
var (
	user32 = windows.NewLazySystemDLL("user32.dll")

	procRegisterClassExW = user32.NewProc("RegisterClassExW")
	procUnregisterClassW = user32.NewProc("UnregisterClassW")
	procCreateWindowExW  = user32.NewProc("CreateWindowExW")
	procDestroyWindow    = user32.NewProc("DestroyWindow")
	procDefWindowProcW   = user32.NewProc("DefWindowProcW")
	procGetMessageW      = user32.NewProc("GetMessageW")
	procTranslateMessage = user32.NewProc("TranslateMessage")
	procDispatchMessageW = user32.NewProc("DispatchMessageW")
	procPostQuitMessage  = user32.NewProc("PostQuitMessage")
	procPostMessageW     = user32.NewProc("PostMessageW")
)

// wndClassExW is the Win32 WNDCLASSEXW layout. Field order MUST match the
// binary layout; reordering breaks RegisterClassExW (Win32 reads the
// struct via its CbSize byte offset and silently rejects mismatched
// layouts). Only the fields we populate are documented below; the rest
// are zero-initialized and ignored by Win32 for the message-only window
// use case.
type wndClassExW struct {
	CbSize        uint32 // MUST be unsafe.Sizeof(wndClassExW{})
	Style         uint32
	LpfnWndProc   uintptr // WndProc callback from syscall.NewCallback
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

// msg is the Win32 MSG layout consumed by GetMessageW / DispatchMessageW.
type msg struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
	// LPrivate omitted; zeros by default and Win32 does not read it back
	// from user code.
}

// point is the Win32 POINT layout used inside MSG.
type point struct {
	X int32
	Y int32
}

// Win32 constants consumed by CreateWindowExW and the message pump.
// HWND_MESSAGE (-3) as uintptr is the message-only window parent, which
// makes the window invisible and off the taskbar — exactly what we want
// for a hidden WndProc host.
const (
	HWND_MESSAGE     = ^uintptr(2) // -3 in two's complement (Win32 HWND_MESSAGE)
	WS_EX_NOACTIVATE = 0x08000000
	CS_HREDRAW       = 0x0002
	CS_VREDRAW       = 0x0001
)
