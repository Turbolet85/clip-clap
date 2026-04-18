package overlay

import (
	"errors"
	"image"
	"testing"
)

func TestDragRect_Normalizes(t *testing.T) {
	// User drags bottom-right → top-left; NormalizeRect must swap so that
	// the returned rectangle has left ≤ right and top ≤ bottom.
	got, err := NormalizeRect(POINT{X: 400, Y: 400}, POINT{X: 100, Y: 100})
	if err != nil {
		t.Fatalf("NormalizeRect returned error: %v", err)
	}
	want := image.Rect(100, 100, 400, 400)
	if got != want {
		t.Errorf("NormalizeRect = %v, want %v", got, want)
	}
}

func TestDragRect_NormalizesMultiple(t *testing.T) {
	cases := []struct {
		name       string
		start, end POINT
		want       image.Rectangle
	}{
		{"top-left → bottom-right", POINT{10, 20}, POINT{30, 40}, image.Rect(10, 20, 30, 40)},
		{"bottom-left → top-right", POINT{10, 40}, POINT{30, 20}, image.Rect(10, 20, 30, 40)},
		{"top-right → bottom-left", POINT{30, 20}, POINT{10, 40}, image.Rect(10, 20, 30, 40)},
		{"single-pixel-wide stripe", POINT{100, 100}, POINT{100, 200}, image.Rect(100, 100, 100, 200)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeRect(tc.start, tc.end)
			if err != nil {
				t.Fatalf("NormalizeRect returned error: %v", err)
			}
			if got != tc.want {
				t.Errorf("NormalizeRect = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDragRect_ZeroSizeRejected(t *testing.T) {
	got, err := NormalizeRect(POINT{X: 200, Y: 200}, POINT{X: 200, Y: 200})
	if !errors.Is(err, ErrDegenerate) {
		t.Errorf("NormalizeRect(zero-size) err = %v, want ErrDegenerate", err)
	}
	if got != (image.Rectangle{}) {
		t.Errorf("NormalizeRect(zero-size) rect = %v, want zero", got)
	}
}
