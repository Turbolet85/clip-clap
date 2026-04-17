package capture

import (
	"fmt"
	"time"
)

// Format returns a capture filename in the architecture-mandated format
// `YYYY-MM-DD_HH-MM-SS_mmm.png` where `mmm` is the zero-padded 3-digit
// millisecond component extracted from t.Nanosecond() / 1_000_000.
//
// The returned string respects t's location (local timezone) — callers should
// pass time.Now() for live captures or a fixed time.Time in tests. This
// function never calls time.Now() itself so tests can inject deterministic
// timestamps per .claude/rules/testing.md.
func Format(t time.Time) string {
	ms := t.Nanosecond() / 1_000_000
	return fmt.Sprintf("%04d-%02d-%02d_%02d-%02d-%02d_%03d.png",
		t.Year(), int(t.Month()), t.Day(),
		t.Hour(), t.Minute(), t.Second(),
		ms)
}
