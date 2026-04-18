// Package overlay owns the transparent full-screen WS_EX_LAYERED window used
// for area-select drag capture. This file declares Win32 constants, lazy-
// loaded procs, and struct layouts NOT exported by golang.org/x/sys/windows
// v0.24.0. See architecture.md §Win32 API Surface for the canonical pattern.
package overlay

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// Window extended styles.
const (
	WS_EX_LAYERED    uint32 = 0x00080000
	WS_EX_TOPMOST    uint32 = 0x00000008
	WS_EX_TOOLWINDOW uint32 = 0x00000080
	WS_EX_NOACTIVATE uint32 = 0x08000000
)

// Window styles.
const (
	WS_POPUP   uint32 = 0x80000000
	WS_VISIBLE uint32 = 0x10000000
)

// ShowWindow commands.
const (
	SW_HIDE     int32 = 0
	SW_SHOW     int32 = 5
	SW_SHOWNA   int32 = 8
	SW_SHOWNOAC int32 = 4
)

// Window messages.
const (
	WM_DESTROY     uint32 = 0x0002
	WM_CLOSE       uint32 = 0x0010
	WM_KEYDOWN     uint32 = 0x0100
	WM_KEYUP       uint32 = 0x0101
	WM_LBUTTONDOWN uint32 = 0x0201
	WM_LBUTTONUP   uint32 = 0x0202
	WM_MOUSEMOVE   uint32 = 0x0200
	WM_PAINT       uint32 = 0x000F
)

// Virtual key codes.
const (
	VK_ESCAPE uint32 = 0x1B
)

// GetSystemMetrics indices for virtual-screen bounds.
const (
	SM_XVIRTUALSCREEN  int32 = 76
	SM_YVIRTUALSCREEN  int32 = 77
	SM_CXVIRTUALSCREEN int32 = 78
	SM_CYVIRTUALSCREEN int32 = 79
)

// UpdateLayeredWindow flags.
const (
	ULW_ALPHA    uint32 = 0x00000002
	ULW_COLORKEY uint32 = 0x00000001
	ULW_OPAQUE   uint32 = 0x00000004
)

// BLENDFUNCTION AC_SRC_* constants.
const (
	AC_SRC_OVER  byte = 0x00
	AC_SRC_ALPHA byte = 0x01
)

// DIB constants.
const (
	BI_RGB         uint32 = 0
	DIB_RGB_COLORS uint32 = 0
)

// BITMAPINFOHEADER — BITMAPINFO header layout.
type BITMAPINFOHEADER struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}

// BITMAPINFO — minimal (no palette) BITMAPINFO for 32-bpp DIB sections.
type BITMAPINFO struct {
	BmiHeader BITMAPINFOHEADER
	BmiColors [1]uint32
}

// BLENDFUNCTION for UpdateLayeredWindow premultiplied-alpha compositing.
type BLENDFUNCTION struct {
	BlendOp             byte
	BlendFlags          byte
	SourceConstantAlpha byte
	AlphaFormat         byte
}

// POINT — GDI 2D point.
type POINT struct {
	X int32
	Y int32
}

// SIZE — GDI width+height struct.
type SIZE struct {
	CX int32
	CY int32
}

// RECT — Win32 rectangle.
type RECT struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

// Window class registration struct (WNDCLASSEXW).
type WNDCLASSEXW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
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

// WNDCLASSEXWSize returns the size to stamp into CbSize.
func WNDCLASSEXWSize() uint32 { return uint32(unsafe.Sizeof(WNDCLASSEXW{})) }

// Error codes.
const (
	ERROR_CLASS_ALREADY_EXISTS = 1410
)

// Lazy-loaded DLLs and procs (x/sys/windows v0.24.0 omits the Win32 UI
// surface; see architecture.md §Win32 API Surface).
var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procRegisterClassExW = user32.NewProc("RegisterClassExW")
	procUnregisterClassW = user32.NewProc("UnregisterClassW")
	procCreateWindowExW  = user32.NewProc("CreateWindowExW")
	procDestroyWindow    = user32.NewProc("DestroyWindow")
	procDefWindowProcW   = user32.NewProc("DefWindowProcW")
	procShowWindow       = user32.NewProc("ShowWindow")
	procUpdateLayeredWin = user32.NewProc("UpdateLayeredWindow")
	procGetSystemMetrics = user32.NewProc("GetSystemMetrics")
	procSetCapture       = user32.NewProc("SetCapture")
	procReleaseCapture   = user32.NewProc("ReleaseCapture")
	procInvalidateRect   = user32.NewProc("InvalidateRect")
	procGetDC            = user32.NewProc("GetDC")
	procReleaseDC        = user32.NewProc("ReleaseDC")
	procGetDpiForWindow  = user32.NewProc("GetDpiForWindow")
	procPostQuitMessage  = user32.NewProc("PostQuitMessage")
	procGetMessageW      = user32.NewProc("GetMessageW")
	procTranslateMessage = user32.NewProc("TranslateMessage")
	procDispatchMessageW = user32.NewProc("DispatchMessageW")

	procCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	procDeleteDC           = gdi32.NewProc("DeleteDC")
	procCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
	procSelectObject       = gdi32.NewProc("SelectObject")
	procDeleteObject       = gdi32.NewProc("DeleteObject")
	procCreateFontW        = gdi32.NewProc("CreateFontW")
	procSetTextColor       = gdi32.NewProc("SetTextColor")
	procSetBkMode          = gdi32.NewProc("SetBkMode")
	procTextOutW           = gdi32.NewProc("TextOutW")
	procGdiFlush           = gdi32.NewProc("GdiFlush")
	procGdiAlphaBlend      = gdi32.NewProc("GdiAlphaBlend")

	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
)

// TRANSPARENT_BK_MODE — argument to SetBkMode.
const TRANSPARENT_BK_MODE int32 = 1
