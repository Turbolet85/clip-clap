package clipboard

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// captureJSONLogger redirects slog output to a buffer for event assertions.
func captureJSONLogger(t *testing.T) *bytes.Buffer {
	t.Helper()
	prev := slog.Default()
	buf := &bytes.Buffer{}
	slog.SetDefault(slog.New(slog.NewJSONHandler(buf, nil)))
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
		var e map[string]any
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("parse log line %q: %v", line, err)
		}
		out = append(out, e)
	}
	return out
}

func TestReentryGuard_PerCaptureID_SilentlyDrops(t *testing.T) {
	// Use an injected fake afterFunc so we can control guard expiration
	// without waiting real time.
	var pending []func()
	SetAfterFunc(func(_ time.Duration, f func()) *time.Timer {
		pending = append(pending, f)
		return nil // caller discards the Timer; no real timer needed
	})
	t.Cleanup(ResetAfterFunc)

	// Install two independent guards.
	installReentryGuard("A")
	installReentryGuard("B")

	buf := captureJSONLogger(t)

	// Both guards are active.
	if !IsGuarded("A") {
		t.Error("guard A should be active immediately after install")
	}
	if !IsGuarded("B") {
		t.Error("guard B should be active immediately after install")
	}

	// No log lines should have been emitted during IsGuarded checks
	// (per security-plan: guard lookup is silent).
	if buf.Len() != 0 {
		t.Errorf("unexpected log output during guard checks: %q", buf.String())
	}

	// Trigger both pending afterFunc callbacks (simulates 500ms elapsing).
	for _, f := range pending {
		f()
	}

	// Both guards are now expired.
	if IsGuarded("A") {
		t.Error("guard A should be expired after afterFunc callback")
	}
	if IsGuarded("B") {
		t.Error("guard B should be expired after afterFunc callback")
	}

	// Still no log lines.
	if buf.Len() != 0 {
		t.Errorf("unexpected log output after guard expiry: %q", buf.String())
	}
}

func TestUndo_EmitsEventWithoutPathOrError(t *testing.T) {
	// This test targets the slog emission contract of Undo: when
	// `lastSnapshot` is nil, Undo is a no-op and emits nothing. When it
	// runs and restores a snapshot, it emits `clipboard.undo` with NO
	// `path` or `error` fields (prior clipboard content is never logged).
	//
	// We can't invoke the real Win32 Undo path without a live clipboard,
	// so this test verifies the NO-OP path + log silence when no snapshot
	// exists. The full Win32 round-trip (snapshot → Swap → Undo) is
	// covered by Phase 4 pywinauto integration tests.
	SetLastSnapshotForTesting(nil)
	buf := captureJSONLogger(t)

	if err := Undo(); err != nil {
		t.Errorf("Undo with nil snapshot: err = %v, want nil", err)
	}
	events := parseEvents(t, buf)
	if len(events) != 0 {
		t.Errorf("Undo with nil snapshot emitted %d events, want 0: %v", len(events), events)
	}
}

func TestUtf16FromString_ASCII(t *testing.T) {
	got, err := utf16FromString("hello")
	if err != nil {
		t.Fatalf("utf16FromString: %v", err)
	}
	want := []uint16{'h', 'e', 'l', 'l', 'o'}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i, c := range want {
		if got[i] != c {
			t.Errorf("[%d] = %d, want %d", i, got[i], c)
		}
	}
}
