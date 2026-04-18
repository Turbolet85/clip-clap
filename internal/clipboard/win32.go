// Package clipboard — Win32 procs and constants for CF_UNICODETEXT
// clipboard writes. Separated from clipboard.go so the pure-logic parts
// (reentry guard, path quoter) are trivially unit-testable.
package clipboard

import "golang.org/x/sys/windows"

// CF_UNICODETEXT — standard clipboard format for UTF-16 text.
const CF_UNICODETEXT uint32 = 13

// GlobalAlloc flags.
const (
	GMEM_MOVEABLE uint32 = 0x0002
	GMEM_ZEROINIT uint32 = 0x0040
)

// Lazy-loaded DLLs.
var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procOpenClipboard    = user32.NewProc("OpenClipboard")
	procCloseClipboard   = user32.NewProc("CloseClipboard")
	procEmptyClipboard   = user32.NewProc("EmptyClipboard")
	procGetClipboardData = user32.NewProc("GetClipboardData")
	procSetClipboardData = user32.NewProc("SetClipboardData")

	procGlobalAlloc   = kernel32.NewProc("GlobalAlloc")
	procGlobalLock    = kernel32.NewProc("GlobalLock")
	procGlobalUnlock  = kernel32.NewProc("GlobalUnlock")
	procGlobalFree    = kernel32.NewProc("GlobalFree")
	procGlobalSize    = kernel32.NewProc("GlobalSize")
	procRtlMoveMemory = kernel32.NewProc("RtlMoveMemory")
)
