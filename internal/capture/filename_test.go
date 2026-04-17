package capture

import (
	"strings"
	"testing"
	"time"
)

// TestFormat_MillisecondZeroPadding exercises the 3-digit zero-padded
// millisecond suffix for the four representative cases from the plan
// (0, 7, 42, 999). Table-driven so adding a boundary case is one-line.
func TestFormat_MillisecondZeroPadding(t *testing.T) {
	cases := []struct {
		name   string
		millis int
		want   string
	}{
		{"zero", 0, "_000.png"},
		{"single digit 7", 7, "_007.png"},
		{"two digits 42", 42, "_042.png"},
		{"max 999", 999, "_999.png"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts := time.Date(2026, time.April, 17, 14, 30, 22, tc.millis*1_000_000, time.Local)
			got := Format(ts)
			if !strings.HasSuffix(got, tc.want) {
				t.Errorf("millisecond zero-padding\n  want suffix: %q\n  got:         %q", tc.want, got)
			}
		})
	}
}

// TestFormat_MidnightBoundary — a timestamp at exact midnight (00:00:00.000)
// should produce _00-00-00_000 in the output.
func TestFormat_MidnightBoundary(t *testing.T) {
	ts := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.Local)
	got := Format(ts)
	if !strings.Contains(got, "_00-00-00_000") {
		t.Errorf("midnight boundary should contain '_00-00-00_000'; got: %q", got)
	}
	if got != "2026-01-01_00-00-00_000.png" {
		t.Errorf("midnight format\n  want: %q\n  got:  %q", "2026-01-01_00-00-00_000.png", got)
	}
}
