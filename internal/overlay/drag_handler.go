package overlay

import (
	"errors"
	"image"
	"sync"
)

// ErrDegenerate is returned by NormalizeRect when start == end (zero-size
// rectangle). The hotkey handler discards degenerate drags without invoking
// the capture pipeline.
var ErrDegenerate = errors.New("overlay: degenerate (zero-size) drag rectangle")

// DragState tracks the live drag rectangle between WM_LBUTTONDOWN and
// WM_LBUTTONUP. Fields are in virtual-screen coordinates (relative to the
// overlay window, which covers the full virtual screen per Step 4).
type DragState struct {
	startPoint POINT
	current    POINT
	capturing  bool
}

var (
	dragMu      sync.Mutex
	currentDrag DragState
)

// HandleLButtonDown records the start point and marks a drag as in progress.
// Safe to call from the Win32 message-pump goroutine.
func HandleLButtonDown(x, y int32) {
	dragMu.Lock()
	defer dragMu.Unlock()
	currentDrag = DragState{
		startPoint: POINT{X: x, Y: y},
		current:    POINT{X: x, Y: y},
		capturing:  true,
	}
}

// HandleMouseMove updates the drag rectangle's current point while a drag is
// active; no-op if no drag is in progress.
func HandleMouseMove(x, y int32) {
	dragMu.Lock()
	defer dragMu.Unlock()
	if !currentDrag.capturing {
		return
	}
	currentDrag.current = POINT{X: x, Y: y}
}

// HandleLButtonUp finalizes the drag and returns the normalized capture
// rectangle along with a bool indicating whether capture should proceed. A
// degenerate (zero-size) drag returns ok=false and is silently discarded.
func HandleLButtonUp(x, y int32) (image.Rectangle, bool) {
	dragMu.Lock()
	defer dragMu.Unlock()
	if !currentDrag.capturing {
		return image.Rectangle{}, false
	}
	start := currentDrag.startPoint
	currentDrag.capturing = false
	rect, err := NormalizeRect(start, POINT{X: x, Y: y})
	if err != nil {
		return image.Rectangle{}, false
	}
	return rect, true
}

// CurrentRect returns the rectangle represented by the in-progress drag
// (unnormalized: left may be greater than right during right-to-left drags).
// Called by the renderer on each WM_MOUSEMOVE repaint.
func CurrentRect() (image.Rectangle, bool) {
	dragMu.Lock()
	defer dragMu.Unlock()
	if !currentDrag.capturing {
		return image.Rectangle{}, false
	}
	return image.Rect(
		int(currentDrag.startPoint.X), int(currentDrag.startPoint.Y),
		int(currentDrag.current.X), int(currentDrag.current.Y),
	).Canon(), true
}

// ResetDragState clears any in-progress drag. Used on Esc dismissal so the
// next overlay invocation starts clean.
func ResetDragState() {
	dragMu.Lock()
	defer dragMu.Unlock()
	currentDrag = DragState{}
}

// NormalizeRect produces a normalized image.Rectangle from two drag endpoints
// (left ≤ right, top ≤ bottom) regardless of drag direction. Returns
// ErrDegenerate if the two points are identical (zero-size selection).
//
// Pure function — no I/O, no Win32 calls. Exposed for unit testing per the
// Phase 3 plan Step 3 Test: block.
func NormalizeRect(start, end POINT) (image.Rectangle, error) {
	if start.X == end.X && start.Y == end.Y {
		return image.Rectangle{}, ErrDegenerate
	}
	x0, x1 := int(start.X), int(end.X)
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	y0, y1 := int(start.Y), int(end.Y)
	if y0 > y1 {
		y0, y1 = y1, y0
	}
	return image.Rect(x0, y0, x1, y1), nil
}
