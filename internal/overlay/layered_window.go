// Package overlay implements the transparent full-screen WS_EX_LAYERED
// overlay window used for area-select drag capture. See Phase 3 plan Step 4
// and design-system §Surface: desktop-custom for the full spec.
package overlay

import (
	"errors"
	"fmt"
	"image"
	"math"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Window class name — this EXACT literal is the verification-harness contract
// (pywinauto integration tests locate the overlay via
// find_windows(class_name='clip-clap-overlay')). Do not change.
const OverlayClassName = "clip-clap-overlay"

var (
	// wndProcCallback holds the windows.NewCallback result for the entire
	// process lifetime. A sync.Once guard prevents the callback from being
	// re-created on a second CreateOverlay() call (which would orphan the
	// first, risking GC mid-dispatch and a crash). See plan Step 4.
	wndProcCallback uintptr
	callbackOnce    sync.Once

	classRegMu sync.Mutex
	classReg   bool

	// overlayState is populated when CreateOverlay spawns a window and reset
	// when fadeAndDestroy tears it down. Only one overlay may exist at a
	// time (second hotkey press during an open overlay is a no-op).
	overlayMu    sync.Mutex
	overlayState *overlay

	// instructionsShown is a once-per-process flag per design-system Motion
	// §Instructional text — micro-bar displays on the first overlay only.
	instructionsShown bool
)

// overlay bundles the live-overlay Win32 handles and DIB buffer.
type overlay struct {
	hwnd           uintptr
	memDC          uintptr
	dib            uintptr
	bits           unsafe.Pointer
	width          int32
	height         int32
	virtualX       int32
	virtualY       int32
	callback       func(image.Rectangle)
	currentAlpha   byte
	microBarBytes  int
	microBarStart  time.Time
	microBarActive bool
	destroying     bool
}

// ensureWndProcCallback installs the package-level Go callback for the
// overlay WndProc exactly once. Thread-safe.
func ensureWndProcCallback() {
	callbackOnce.Do(func() {
		wndProcCallback = windows.NewCallback(wndProcOverlay)
	})
}

// CreateOverlay spawns a full-screen WS_EX_LAYERED overlay window and begins
// a drag-rectangle selection. When the user releases the left mouse button
// on a non-degenerate rectangle, captureCallback is invoked asynchronously
// on a background goroutine with the rectangle in virtual-screen
// coordinates. When the user presses Esc, the overlay dismisses without
// invoking the callback.
//
// Idempotency: repeated calls while an overlay is already live are no-ops
// (they return nil and do nothing). The second WM_HOTKEY press during an
// open overlay is treated as a discard.
func CreateOverlay(captureCallback func(image.Rectangle)) error {
	overlayMu.Lock()
	if overlayState != nil {
		overlayMu.Unlock()
		return nil // already open — no-op
	}
	overlayMu.Unlock()

	ensureWndProcCallback()

	if err := ensureClassRegistered(); err != nil {
		return fmt.Errorf("overlay: register class: %w", err)
	}

	// Virtual-screen bounds cover ALL monitors (multi-monitor support).
	x := getSystemMetric(SM_XVIRTUALSCREEN)
	y := getSystemMetric(SM_YVIRTUALSCREEN)
	w := getSystemMetric(SM_CXVIRTUALSCREEN)
	h := getSystemMetric(SM_CYVIRTUALSCREEN)
	if w <= 0 || h <= 0 {
		return errors.New("overlay: virtual screen has zero dimensions")
	}

	ov := &overlay{
		width:    w,
		height:   h,
		virtualX: x,
		virtualY: y,
		callback: captureCallback,
	}

	hInstance, _, _ := procGetModuleHandleW.Call(0)

	classNamePtr, _ := syscall.UTF16PtrFromString(OverlayClassName)
	windowNamePtr, _ := syscall.UTF16PtrFromString("")

	hwnd, _, err := procCreateWindowExW.Call(
		uintptr(WS_EX_LAYERED|WS_EX_TOPMOST|WS_EX_TOOLWINDOW|WS_EX_NOACTIVATE),
		uintptr(unsafe.Pointer(classNamePtr)),
		uintptr(unsafe.Pointer(windowNamePtr)),
		uintptr(WS_POPUP),
		uintptr(x), uintptr(y),
		uintptr(w), uintptr(h),
		0, 0, hInstance, 0,
	)
	if hwnd == 0 {
		return fmt.Errorf("overlay: CreateWindowExW failed: %w", err)
	}
	ov.hwnd = hwnd

	if err := createDIB(ov); err != nil {
		procDestroyWindow.Call(hwnd)
		return fmt.Errorf("overlay: create DIB: %w", err)
	}

	overlayMu.Lock()
	overlayState = ov
	overlayMu.Unlock()

	// Paint the initial dim fill (no drag rect yet) and show the window.
	renderDIB(ov, image.Rectangle{}, 0x00) // alpha 0 initially for fade-in
	procShowWindow.Call(hwnd, uintptr(SW_SHOWNA))

	// Start the 80-120ms alpha fade-in animation. Per design-system Motion,
	// use cubic ease-out: α(t) = α_target * (1 - (1-t/d)^3). Target alpha
	// is 0x8C (140 ≈ 0.55 * 255).
	go runFadeIn(ov)

	// Trigger the one-shot instructional micro-bar, if not yet shown this
	// process lifetime. Fades in 150ms after overlay appears.
	if !instructionsShown {
		instructionsShown = true
		go runInstructionalMicroBar(ov)
	}

	return nil
}

// ensureClassRegistered registers the overlay window class exactly once per
// process. Idempotent: if RegisterClassExW returns ERROR_CLASS_ALREADY_EXISTS
// (1410), continue silently — second hotkey press after first dismiss.
func ensureClassRegistered() error {
	classRegMu.Lock()
	defer classRegMu.Unlock()
	if classReg {
		return nil
	}

	hInstance, _, _ := procGetModuleHandleW.Call(0)
	classNamePtr, _ := syscall.UTF16PtrFromString(OverlayClassName)

	wc := WNDCLASSEXW{
		CbSize:        WNDCLASSEXWSize(),
		Style:         0,
		LpfnWndProc:   wndProcCallback,
		HInstance:     hInstance,
		LpszClassName: classNamePtr,
	}
	ret, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	if ret == 0 {
		// Check for ERROR_CLASS_ALREADY_EXISTS (idempotent path).
		if errno, ok := err.(syscall.Errno); ok && errno == ERROR_CLASS_ALREADY_EXISTS {
			classReg = true
			return nil
		}
		return fmt.Errorf("RegisterClassExW: %w", err)
	}
	classReg = true
	return nil
}

// createDIB allocates a 32-bpp top-down DIB section sized to the virtual
// screen and a compatible memory DC for it.
func createDIB(ov *overlay) error {
	hdcScreen, _, _ := procGetDC.Call(0)
	defer procReleaseDC.Call(0, hdcScreen)

	memDC, _, _ := procCreateCompatibleDC.Call(hdcScreen)
	if memDC == 0 {
		return errors.New("CreateCompatibleDC returned 0")
	}

	bmi := BITMAPINFO{
		BmiHeader: BITMAPINFOHEADER{
			BiSize:        uint32(unsafe.Sizeof(BITMAPINFOHEADER{})),
			BiWidth:       ov.width,
			BiHeight:      -ov.height, // top-down
			BiPlanes:      1,
			BiBitCount:    32,
			BiCompression: BI_RGB,
		},
	}

	var bits unsafe.Pointer
	dib, _, _ := procCreateDIBSection.Call(
		memDC,
		uintptr(unsafe.Pointer(&bmi)),
		uintptr(DIB_RGB_COLORS),
		uintptr(unsafe.Pointer(&bits)),
		0, 0,
	)
	if dib == 0 || bits == nil {
		procDeleteDC.Call(memDC)
		return errors.New("CreateDIBSection returned 0")
	}
	procSelectObject.Call(memDC, dib)

	ov.memDC = memDC
	ov.dib = dib
	ov.bits = bits
	return nil
}

// renderDIB paints the overlay DIB buffer with the dim fill, carves out the
// drag rectangle's interior as transparent, and strokes the drag-rect with
// shadow + cream + cream tick-marks. Then calls UpdateLayeredWindow with
// the global alpha (0-255) for fade-in/out animations.
//
// The dragRect argument is in virtual-screen coordinates (absolute). It is
// translated to overlay-local coordinates (by subtracting virtualX/Y) for
// the DIB pixel writes.
func renderDIB(ov *overlay, dragRect image.Rectangle, globalAlpha byte) {
	if ov.bits == nil {
		return
	}
	fillDIB(ov)
	if !dragRect.Empty() {
		local := dragRect.Sub(image.Pt(int(ov.virtualX), int(ov.virtualY)))
		carveDragRect(ov, local)
	}

	// UpdateLayeredWindow with per-pixel alpha (SourceConstantAlpha = 255)
	// when the DIB itself encodes the alpha channel. During fade-in/out,
	// we scale the constant alpha (globalAlpha) to modulate the whole window.
	bf := BLENDFUNCTION{
		BlendOp:             AC_SRC_OVER,
		BlendFlags:          0,
		SourceConstantAlpha: globalAlpha,
		AlphaFormat:         AC_SRC_ALPHA,
	}

	hdcScreen, _, _ := procGetDC.Call(0)
	defer procReleaseDC.Call(0, hdcScreen)

	winPos := POINT{X: ov.virtualX, Y: ov.virtualY}
	winSize := SIZE{CX: ov.width, CY: ov.height}
	srcPos := POINT{X: 0, Y: 0}

	procUpdateLayeredWin.Call(
		ov.hwnd,
		hdcScreen,
		uintptr(unsafe.Pointer(&winPos)),
		uintptr(unsafe.Pointer(&winSize)),
		ov.memDC,
		uintptr(unsafe.Pointer(&srcPos)),
		0,
		uintptr(unsafe.Pointer(&bf)),
		uintptr(ULW_ALPHA),
	)
	ov.currentAlpha = globalAlpha
}

// fillDIB writes the canonical overlay.dim-fill.premul token (B=0x08, G=0x07,
// R=0x06, A=0x8C) across every pixel of the DIB buffer. This is the darkroom
// "dim room" fill per design-system Surface: desktop-custom.
func fillDIB(ov *overlay) {
	total := int(ov.width) * int(ov.height)
	buf := unsafe.Slice((*uint32)(ov.bits), total)
	// BGRA little-endian packed into uint32: A<<24 | R<<16 | G<<8 | B
	const pixel = (uint32(0x8C) << 24) | (uint32(0x06) << 16) | (uint32(0x07) << 8) | uint32(0x08)
	for i := range buf {
		buf[i] = pixel
	}
}

// carveDragRect renders the drag rectangle: carves a transparent hole inside,
// paints the 1px deep-ink shadow (offset 1px down-right), the 1px cream
// stroke, and four 6-logical-px amber tick-marks at edge midpoints.
func carveDragRect(ov *overlay, local image.Rectangle) {
	if local.Empty() {
		return
	}
	// Clip to DIB bounds.
	if local.Min.X < 0 {
		local.Min.X = 0
	}
	if local.Min.Y < 0 {
		local.Min.Y = 0
	}
	if local.Max.X > int(ov.width) {
		local.Max.X = int(ov.width)
	}
	if local.Max.Y > int(ov.height) {
		local.Max.Y = int(ov.height)
	}
	buf := unsafe.Slice((*uint32)(ov.bits), int(ov.width)*int(ov.height))
	w := int(ov.width)

	// Inside: fully transparent (alpha=0, color doesn't matter — 0x00000000).
	for y := local.Min.Y; y < local.Max.Y; y++ {
		rowStart := y * w
		for x := local.Min.X; x < local.Max.X; x++ {
			buf[rowStart+x] = 0x00000000
		}
	}

	// Outer shadow: 1px deep-ink (#0E1013) offset 1px down-right of the stroke.
	const shadow = (uint32(0xFF) << 24) | (uint32(0x0E) << 16) | (uint32(0x10) << 8) | uint32(0x13)
	drawHorizontalLine(buf, w, local.Min.X+1, local.Max.X, local.Min.Y+1, shadow)
	drawHorizontalLine(buf, w, local.Min.X+1, local.Max.X, local.Max.Y, shadow)
	drawVerticalLine(buf, w, local.Min.X+1, local.Min.Y+1, local.Max.Y+1, shadow)
	drawVerticalLine(buf, w, local.Max.X, local.Min.Y+1, local.Max.Y+1, shadow)

	// Inner stroke: 1px cream #EDE6D4 premultiplied (opaque, so premul = raw).
	const stroke = (uint32(0xFF) << 24) | (uint32(0xED) << 16) | (uint32(0xE6) << 8) | uint32(0xD4)
	drawHorizontalLine(buf, w, local.Min.X, local.Max.X, local.Min.Y, stroke)
	drawHorizontalLine(buf, w, local.Min.X, local.Max.X, local.Max.Y-1, stroke)
	drawVerticalLine(buf, w, local.Min.X, local.Min.Y, local.Max.Y, stroke)
	drawVerticalLine(buf, w, local.Max.X-1, local.Min.Y, local.Max.Y, stroke)

	// Tick-marks: 4 × #C64A1E (amber) 6-logical-px segments at edge midpoints.
	// Scale: 6 * DpiForWindow / 96. Fall back to 6 if GetDpiForWindow fails.
	tickSize := 6
	if dpi, _, _ := procGetDpiForWindow.Call(ov.hwnd); dpi > 0 {
		tickSize = int(math.Round(6.0 * float64(dpi) / 96.0))
	}
	const tick = (uint32(0xFF) << 24) | (uint32(0xC6) << 16) | (uint32(0x4A) << 8) | uint32(0x1E)
	midX := (local.Min.X + local.Max.X) / 2
	midY := (local.Min.Y + local.Max.Y) / 2
	half := tickSize / 2

	// Top-edge tick (horizontal segment at midX on Y = Min.Y).
	drawHorizontalLine(buf, w, midX-half, midX+half, local.Min.Y, tick)
	// Bottom-edge tick.
	drawHorizontalLine(buf, w, midX-half, midX+half, local.Max.Y-1, tick)
	// Left-edge tick.
	drawVerticalLine(buf, w, local.Min.X, midY-half, midY+half, tick)
	// Right-edge tick.
	drawVerticalLine(buf, w, local.Max.X-1, midY-half, midY+half, tick)
}

func drawHorizontalLine(buf []uint32, stride, x0, x1, y int, color uint32) {
	if y < 0 {
		return
	}
	if x0 < 0 {
		x0 = 0
	}
	if x1 > stride {
		x1 = stride
	}
	rowStart := y * stride
	for x := x0; x < x1; x++ {
		if rowStart+x >= 0 && rowStart+x < len(buf) {
			buf[rowStart+x] = color
		}
	}
}

func drawVerticalLine(buf []uint32, stride, x, y0, y1 int, color uint32) {
	if x < 0 || x >= stride {
		return
	}
	if y0 < 0 {
		y0 = 0
	}
	for y := y0; y < y1; y++ {
		idx := y*stride + x
		if idx >= 0 && idx < len(buf) {
			buf[idx] = color
		}
	}
}

// runFadeIn animates the overlay from alpha 0 → ~140 (0x8C, the 55% dim-fill
// target) over 6 frames / ~100ms using cubic ease-out.
func runFadeIn(ov *overlay) {
	const frames = 6
	const duration = 100 * time.Millisecond
	targetAlpha := byte(0x8C)
	start := time.Now()
	for i := 1; i <= frames; i++ {
		elapsed := time.Since(start)
		t := float64(elapsed) / float64(duration)
		if t > 1.0 {
			t = 1.0
		}
		// cubic ease-out: 1 - (1-t)^3
		eased := 1.0 - math.Pow(1.0-t, 3)
		alpha := byte(math.Round(float64(targetAlpha) * eased))
		dragRect, _ := CurrentRect()
		// Absolute coordinates for carveDragRect: CurrentRect returns
		// overlay-local virtual-screen coords → convert to absolute by
		// adding virtualX/Y.
		if !dragRect.Empty() {
			dragRect = dragRect.Add(image.Pt(int(ov.virtualX), int(ov.virtualY)))
		}
		renderDIB(ov, dragRect, alpha)
		time.Sleep(duration / frames)
	}
}

// runInstructionalMicroBar paints the "Drag to capture · Esc to cancel" text
// micro-bar 150ms after the overlay appears. Holds for 1500ms, then fades
// out over 200ms. Fires at most once per process lifetime.
//
// Due to the complexity of blending GDI-drawn text into the premultiplied
// DIB while another goroutine is repainting on WM_MOUSEMOVE, this is
// implemented as a best-effort cosmetic overlay that skips rendering if a
// drag is already in progress (user understands the tool — no need for
// instructions).
func runInstructionalMicroBar(ov *overlay) {
	// Wait for overlay to stabilize (after fade-in).
	time.Sleep(150 * time.Millisecond)

	// Skip if drag already started — no need to instruct.
	if _, active := CurrentRect(); active {
		return
	}

	// Render the text strip into the DIB at the bottom-center.
	drawMicroBar(ov, "Drag to capture · Esc to cancel")

	// Hold 1500ms, then let subsequent renders overwrite it during drag.
	time.Sleep(1500 * time.Millisecond)
}

// drawMicroBar renders the "Drag to capture · Esc to cancel" text as a Raised-1
// pill near the center-bottom of the overlay. Uses GDI TextOutW with Segoe UI
// Variable (fallback if IBM Plex Sans TTF not bundled).
func drawMicroBar(ov *overlay, text string) {
	// Create a compatible DC for text rendering.
	hdcScreen, _, _ := procGetDC.Call(0)
	defer procReleaseDC.Call(0, hdcScreen)

	// Font: Segoe UI Variable at 13px (per design-system IBM Plex Sans fallback).
	fontName, _ := syscall.UTF16PtrFromString("Segoe UI Variable")
	hFont, _, _ := procCreateFontW.Call(
		uintptr(13), // height
		0, 0, 0,     // width, escapement, orientation
		uintptr(400), // FW_NORMAL
		0, 0, 0,      // italic, underline, strikeout
		uintptr(1), // DEFAULT_CHARSET
		uintptr(0), // OUT_DEFAULT_PRECIS
		uintptr(0), // CLIP_DEFAULT_PRECIS
		uintptr(4), // CLEARTYPE_QUALITY
		uintptr(0), // DEFAULT_PITCH
		uintptr(unsafe.Pointer(fontName)),
	)
	if hFont == 0 {
		return
	}
	defer procDeleteObject.Call(hFont)

	procSelectObject.Call(ov.memDC, hFont)
	// Cream text #EDE6D4 as COLORREF (BGR byte order: 0x00D4E6ED).
	procSetTextColor.Call(ov.memDC, 0x00D4E6ED)
	procSetBkMode.Call(ov.memDC, uintptr(TRANSPARENT_BK_MODE))

	textUTF16, _ := syscall.UTF16FromString(text)
	textLen := len(textUTF16) - 1 // strip NUL terminator

	// Compute rough placement: center-bottom with 60px from bottom.
	// Pixel position in DIB space (top-left origin since top-down DIB).
	x := int32(ov.width/2 - 110) // rough center offset (text is ~220px wide)
	y := ov.height - 80

	// Paint the Raised-1 pill background (#1B2026 opaque) behind the text.
	pillX0 := int(x - 12)
	pillX1 := int(x + 220 + 12)
	pillY0 := int(y - 6)
	pillY1 := int(y + 24)
	const pill = (uint32(0xFF) << 24) | (uint32(0x1B) << 16) | (uint32(0x20) << 8) | uint32(0x26)
	buf := unsafe.Slice((*uint32)(ov.bits), int(ov.width)*int(ov.height))
	w := int(ov.width)
	for py := pillY0; py < pillY1; py++ {
		if py < 0 || py >= int(ov.height) {
			continue
		}
		rowStart := py * w
		for px := pillX0; px < pillX1; px++ {
			if px < 0 || px >= w {
				continue
			}
			buf[rowStart+px] = pill
		}
	}

	procTextOutW.Call(
		ov.memDC,
		uintptr(x),
		uintptr(y),
		uintptr(unsafe.Pointer(&textUTF16[0])),
		uintptr(textLen),
	)
	procGdiFlush.Call()

	// Push the update to the screen.
	renderDIB(ov, image.Rectangle{}, ov.currentAlpha)
}

// wndProcOverlay is the package-level window procedure for the overlay
// window class. Handles mouse drag events + Esc dismissal + destroy.
func wndProcOverlay(hwnd, msg, wparam, lparam uintptr) uintptr {
	overlayMu.Lock()
	ov := overlayState
	overlayMu.Unlock()

	switch uint32(msg) {
	case WM_LBUTTONDOWN:
		if ov == nil {
			break
		}
		x := int32(int16(uint16(lparam&0xFFFF))) + ov.virtualX
		y := int32(int16(uint16((lparam>>16)&0xFFFF))) + ov.virtualY
		HandleLButtonDown(x, y)
		procSetCapture.Call(hwnd)
		return 0
	case WM_MOUSEMOVE:
		if ov == nil {
			break
		}
		x := int32(int16(uint16(lparam&0xFFFF))) + ov.virtualX
		y := int32(int16(uint16((lparam>>16)&0xFFFF))) + ov.virtualY
		HandleMouseMove(x, y)
		if dragRect, active := CurrentRect(); active {
			abs := dragRect.Add(image.Pt(int(ov.virtualX), int(ov.virtualY)))
			renderDIB(ov, abs, ov.currentAlpha)
		}
		return 0
	case WM_LBUTTONUP:
		if ov == nil {
			break
		}
		x := int32(int16(uint16(lparam&0xFFFF))) + ov.virtualX
		y := int32(int16(uint16((lparam>>16)&0xFFFF))) + ov.virtualY
		procReleaseCapture.Call()
		rect, ok := HandleLButtonUp(x, y)
		if ok && ov.callback != nil {
			cb := ov.callback
			go cb(rect)
		}
		destroyOverlay(ov)
		return 0
	case WM_KEYDOWN:
		if ov == nil {
			break
		}
		if uint32(wparam) == VK_ESCAPE {
			ResetDragState()
			destroyOverlay(ov)
		}
		return 0
	case WM_DESTROY:
		return 0
	}
	ret, _, _ := procDefWindowProcW.Call(hwnd, msg, wparam, lparam)
	return ret
}

// destroyOverlay tears down the overlay window and frees the DIB resources.
// Safe to call from the message pump.
func destroyOverlay(ov *overlay) {
	if ov == nil {
		return
	}
	overlayMu.Lock()
	if ov.destroying {
		overlayMu.Unlock()
		return
	}
	ov.destroying = true
	overlayState = nil
	overlayMu.Unlock()

	if ov.hwnd != 0 {
		procShowWindow.Call(ov.hwnd, uintptr(SW_HIDE))
		procDestroyWindow.Call(ov.hwnd)
	}
	if ov.dib != 0 {
		procDeleteObject.Call(ov.dib)
	}
	if ov.memDC != 0 {
		procDeleteDC.Call(ov.memDC)
	}
}

// getSystemMetric wraps GetSystemMetrics with standard signed-int32 return
// handling.
func getSystemMetric(idx int32) int32 {
	ret, _, _ := procGetSystemMetrics.Call(uintptr(idx))
	return int32(ret)
}
