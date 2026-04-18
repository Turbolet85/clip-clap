package status

import "testing"

// Each test resets package-level state first because the state lives in
// a package-global `packageState` and all tests in this file share it.
// Without the reset, test order would determine the outcome of IsReady
// (and worse — tests could leak state into the downstream handler_test.go
// suite). The `resetForTesting` helper is package-private and defined in
// state.go.

func TestStateMarkReady(t *testing.T) {
	resetForTesting()
	if IsReady() {
		t.Fatalf("IsReady should be false before MarkReady")
	}
	MarkReady()
	if !IsReady() {
		t.Fatalf("IsReady should be true after MarkReady")
	}
}

func TestStateSetLastCapture(t *testing.T) {
	resetForTesting()
	if got := GetLastCapture(); got != "" {
		t.Fatalf("GetLastCapture should be empty initially, got %q", got)
	}
	SetLastCapture("test.png")
	if got := GetLastCapture(); got != "test.png" {
		t.Fatalf("GetLastCapture = %q, want %q", got, "test.png")
	}
	SetLastCapture("replaced.png")
	if got := GetLastCapture(); got != "replaced.png" {
		t.Fatalf("GetLastCapture after overwrite = %q, want %q", got, "replaced.png")
	}
}

func TestStateBeginShutdown(t *testing.T) {
	resetForTesting()
	if IsShutdown() {
		t.Fatalf("IsShutdown should be false before BeginShutdown")
	}
	BeginShutdown()
	if !IsShutdown() {
		t.Fatalf("IsShutdown should be true after BeginShutdown")
	}
}
