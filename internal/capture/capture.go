// Package capture orchestrates the screen-capture pipeline: kbinani/screenshot
// CaptureRect, PNG encode, save_folder ensure-exists, filename formatting.
// See Phase 3 plan Step 7 and architecture.md §Clipboard Trigger Model for
// the canonical flow.
package capture

import (
	"crypto/rand"
	"errors"
	"fmt"
	"image"
	"image/png"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/kbinani/screenshot"
	"github.com/oklog/ulid/v2"

	"github.com/Turbolet85/clip-clap/internal/lasterror"
	"github.com/Turbolet85/clip-clap/internal/logger"
	"github.com/Turbolet85/clip-clap/internal/tray"
)

// Module-level entropy source for ULID generation. Declared at package scope
// (NOT inside a function or func init()) so it is initialized EXACTLY ONCE
// when the package loads — monotonicity is guaranteed only with a single
// long-lived entropy source. Seeded with crypto/rand.Reader per security-plan
// §Win32 Resource Hygiene (Step 7 Security).
var (
	entropyMu sync.Mutex
	entropy   = ulid.Monotonic(rand.Reader, 0)
)

// ScreenGrabber abstracts kbinani/screenshot.CaptureRect for dependency
// injection in unit tests. Production uses DefaultGrabber which calls the
// real library; tests substitute a stub that returns deterministic bytes.
type ScreenGrabber interface {
	CaptureRect(rect image.Rectangle) (*image.RGBA, error)
}

// DefaultGrabber wraps kbinani/screenshot.CaptureRect for live captures.
type DefaultGrabber struct{}

// CaptureRect implements ScreenGrabber.
func (DefaultGrabber) CaptureRect(rect image.Rectangle) (*image.RGBA, error) {
	return screenshot.CaptureRect(rect)
}

// Package-level grabber — overridable via SetGrabber in tests.
var grabber ScreenGrabber = DefaultGrabber{}

// SetGrabber swaps the package-level ScreenGrabber. Call in tests BEFORE
// invoking Capture; restore via t.Cleanup.
func SetGrabber(g ScreenGrabber) { grabber = g }

// Capture runs the full capture pipeline for a single drag-rectangle
// selection: generate capture_id, ensure save folder exists, grab the
// screen, PNG-encode to disk, and emit structured slog events for every
// stage. Returns the generated capture_id and the absolute path of the
// saved PNG.
//
// The `clock` parameter is injectable for deterministic testing — pass
// time.Now in production, a fixed-time function in tests.
//
// Events emitted (via slog to the package-level handler):
//   - `capture.started` with `capture_id` — always fires before work begins
//   - `capture.completed` with `capture_id` + `path` — on success
//   - `capture.failed` with `capture_id` + `error` — on any failure
//
// Errors are wrapped via fmt.Errorf("%w", err) and also surfaced to the
// tray Last-error slot via tray.SanitizeForTray(err) → lasterror.Set.
// The *os.PathError path field is basename-only in logs per security-plan.
func Capture(rect image.Rectangle, saveFolder string, clock func() time.Time) (captureID, absPath string, err error) {
	now := clock()
	captureID = newULID(now)

	slog.Info("capture started",
		"event", logger.EventCaptureStarted,
		"capture_id", captureID,
	)

	defer func() {
		if err != nil {
			slog.Error("capture failed",
				"event", logger.EventCaptureFailed,
				"capture_id", captureID,
				"save_folder", filepath.Base(saveFolder),
				"error", err.Error(),
			)
			lasterror.Set(tray.SanitizeForTray(err))
		}
	}()

	// Auto-create the save folder (idempotent). Mode bits are ignored on
	// NTFS per security-plan; we pass 0o755 anyway for go vet + cross-
	// platform consistency.
	if mkErr := os.MkdirAll(saveFolder, 0o755); mkErr != nil {
		err = fmt.Errorf("create save folder: %w", mkErr)
		return "", "", err
	}

	img, grabErr := grabber.CaptureRect(rect)
	if grabErr != nil {
		err = fmt.Errorf("capture rect: %w", grabErr)
		return captureID, "", err
	}
	if img == nil {
		err = errors.New("screen grabber returned nil image")
		return captureID, "", err
	}

	filename := Format(now)
	absPath = filepath.Join(saveFolder, filename)

	f, openErr := os.Create(absPath)
	if openErr != nil {
		err = fmt.Errorf("open output: %w", openErr)
		return captureID, "", err
	}
	if encErr := png.Encode(f, img); encErr != nil {
		_ = f.Close()
		_ = os.Remove(absPath)
		err = fmt.Errorf("encode png: %w", encErr)
		return captureID, "", err
	}
	if closeErr := f.Close(); closeErr != nil {
		err = fmt.Errorf("close output: %w", closeErr)
		return captureID, "", err
	}

	slog.Info("capture completed",
		"event", logger.EventCaptureCompleted,
		"capture_id", captureID,
		"path", absPath,
	)
	return captureID, absPath, nil
}

// newULID produces a 26-char Crockford-base32 ULID using the module-level
// monotonic entropy source. The entropyMu mutex serializes entropy access
// (ulid.Monotonic is NOT goroutine-safe by itself per its docs).
func newULID(t time.Time) string {
	entropyMu.Lock()
	defer entropyMu.Unlock()
	return ulid.MustNew(ulid.Timestamp(t), entropy).String()
}
