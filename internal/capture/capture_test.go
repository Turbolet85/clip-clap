package capture

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeGrabber returns a fixed-size RGBA image for deterministic tests.
type fakeGrabber struct {
	img *image.RGBA
	err error
}

func (f fakeGrabber) CaptureRect(_ image.Rectangle) (*image.RGBA, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.img, nil
}

// captureJSONLogger redirects slog output to a buffer of JSON objects for
// event-stream assertions.
func captureJSONLogger(t *testing.T) *bytes.Buffer {
	t.Helper()
	prev := slog.Default()
	buf := &bytes.Buffer{}
	slog.SetDefault(slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return buf
}

func parseEvents(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range strings.Split(buf.String(), "\n") {
		if line == "" {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("failed to parse log line %q: %v", line, err)
		}
		out = append(out, entry)
	}
	return out
}

func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }

func TestCapture_PNGEncoded_ToFilenameFormat(t *testing.T) {
	// 100x80 fully-red RGBA image (color doesn't matter — assert on dims).
	img := image.NewRGBA(image.Rect(0, 0, 100, 80))
	for i := range img.Pix {
		img.Pix[i] = 0xFF
	}
	SetGrabber(fakeGrabber{img: img})
	t.Cleanup(func() { SetGrabber(DefaultGrabber{}) })

	captureJSONLogger(t)
	fixed := time.Date(2026, 4, 17, 14, 30, 22, 481_000_000, time.UTC)
	saveFolder := t.TempDir()
	t.Cleanup(func() { _ = os.RemoveAll(saveFolder) })

	_, path, err := Capture(image.Rect(0, 0, 100, 80), saveFolder, fixedClock(fixed))
	if err != nil {
		t.Fatalf("Capture returned error: %v", err)
	}
	if !strings.HasSuffix(path, "2026-04-17_14-30-22_481.png") {
		t.Errorf("path = %q, want suffix 2026-04-17_14-30-22_481.png", path)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open saved PNG: %v", err)
	}
	defer f.Close()
	decoded, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode saved PNG: %v", err)
	}
	if got := decoded.Bounds(); got != image.Rect(0, 0, 100, 80) {
		t.Errorf("decoded bounds = %v, want 0,0,100,80", got)
	}
}

func TestCapture_AutoCreatesSaveFolder(t *testing.T) {
	SetGrabber(fakeGrabber{img: image.NewRGBA(image.Rect(0, 0, 10, 10))})
	t.Cleanup(func() { SetGrabber(DefaultGrabber{}) })
	captureJSONLogger(t)

	root := t.TempDir()
	saveFolder := filepath.Join(root, "does", "not", "exist", "yet")

	_, path, err := Capture(image.Rect(0, 0, 10, 10), saveFolder, fixedClock(time.Now()))
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if _, err := os.Stat(saveFolder); err != nil {
		t.Errorf("save folder not created: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("PNG not written at %q: %v", path, err)
	}
}

func TestCapture_EmitsStartedAndCompletedWithSameULID(t *testing.T) {
	SetGrabber(fakeGrabber{img: image.NewRGBA(image.Rect(0, 0, 10, 10))})
	t.Cleanup(func() { SetGrabber(DefaultGrabber{}) })

	buf := captureJSONLogger(t)
	saveFolder := t.TempDir()
	captureID, _, err := Capture(image.Rect(0, 0, 10, 10), saveFolder, fixedClock(time.Now()))
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}

	events := parseEvents(t, buf)
	var startedCount, completedCount int
	for _, e := range events {
		switch e["event"] {
		case "capture.started":
			startedCount++
			if e["capture_id"] != captureID {
				t.Errorf("capture.started capture_id = %v, want %q", e["capture_id"], captureID)
			}
		case "capture.completed":
			completedCount++
			if e["capture_id"] != captureID {
				t.Errorf("capture.completed capture_id = %v, want %q", e["capture_id"], captureID)
			}
		}
	}
	if startedCount != 1 {
		t.Errorf("capture.started count = %d, want 1", startedCount)
	}
	if completedCount != 1 {
		t.Errorf("capture.completed count = %d, want 1", completedCount)
	}
	if len(captureID) != 26 {
		t.Errorf("capture_id len = %d, want 26 (ULID)", len(captureID))
	}
}

func TestCapture_EncodeFailureEmitsCaptureFailed(t *testing.T) {
	SetGrabber(fakeGrabber{err: errors.New("simulated grabber failure")})
	t.Cleanup(func() { SetGrabber(DefaultGrabber{}) })

	buf := captureJSONLogger(t)
	saveFolder := t.TempDir()
	captureID, _, err := Capture(image.Rect(0, 0, 10, 10), saveFolder, fixedClock(time.Now()))
	if err == nil {
		t.Fatal("Capture returned nil error, want non-nil")
	}

	events := parseEvents(t, buf)
	var failedCount, completedCount int
	for _, e := range events {
		switch e["event"] {
		case "capture.failed":
			failedCount++
			if e["capture_id"] != captureID {
				t.Errorf("capture.failed capture_id = %v, want %q", e["capture_id"], captureID)
			}
			if errStr, _ := e["error"].(string); errStr == "" {
				t.Error("capture.failed missing error field")
			}
		case "capture.completed":
			completedCount++
		}
	}
	if failedCount != 1 {
		t.Errorf("capture.failed count = %d, want 1", failedCount)
	}
	if completedCount != 0 {
		t.Errorf("capture.completed count = %d, want 0 on failure path", completedCount)
	}
}
