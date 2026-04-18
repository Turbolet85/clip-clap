// Package status — shared package-level state for the loopback-only
// HTTP /status endpoint. Used by handler.go to surface `ready` /
// `last_capture` in JSON responses and by main.go's WM_CLOSE handler
// to trigger the draining window via BeginShutdown.
//
// Concurrency model:
//   - `ready` is an atomic.Bool (flipped once by MarkReady after the
//     listener binds + readyDelay expires; read on every GET /status).
//   - `shutdownFlag` is an atomic.Bool (flipped once by BeginShutdown;
//     read on every GET /status to serve 503 during the draining
//     window before server.Shutdown closes the listener).
//   - `lastCapture` is a string guarded by a sync.RWMutex — writers
//     (capture flow) take the Lock, the /status handler takes RLock.
//     A mutex (not atomic.Value) keeps the API ergonomic with string
//     types and avoids an unnecessary interface{} cast.
package status

import (
	"sync"
	"sync/atomic"
)

// stateStruct groups all three fields together so the "everything the
// handler reads" contract is obvious at the call site. Never exported —
// other packages access this via the package-level helpers below.
type stateStruct struct {
	ready        atomic.Bool
	shutdownFlag atomic.Bool

	mu          sync.RWMutex
	lastCapture string
}

// packageState is the single package-level state instance. All exported
// helpers below operate on this instance; no caller should construct
// their own stateStruct.
var packageState stateStruct

// MarkReady flips the atomic ready flag to true. Called once from the
// Initialize goroutine after time.Sleep(readyDelay) elapses. Idempotent.
func MarkReady() {
	packageState.ready.Store(true)
}

// IsReady returns the current value of the ready flag. Called on every
// GET /status. Atomic load — no locking required.
func IsReady() bool {
	return packageState.ready.Load()
}

// BeginShutdown flips the shutdown flag to true. Called by Shutdown()
// before server.Shutdown(ctx) runs, so /status handlers returning 503
// can start draining. Idempotent.
func BeginShutdown() {
	packageState.shutdownFlag.Store(true)
}

// IsShutdown returns whether BeginShutdown has been called. Used by the
// /status handler to gate 200 vs 503 responses.
func IsShutdown() bool {
	return packageState.shutdownFlag.Load()
}

// SetLastCapture stores the absolute (or relative) path of the most
// recent capture. The handler extracts the basename via filepath.Base
// before emitting to JSON — see handler.go for the rationale (security
// plan §Error Handling: never leak user paths).
func SetLastCapture(path string) {
	packageState.mu.Lock()
	packageState.lastCapture = path
	packageState.mu.Unlock()
}

// GetLastCapture returns the raw stored path (full path, not basename).
// Callers in handler.go must apply filepath.Base before emitting to the
// response body.
func GetLastCapture() string {
	packageState.mu.RLock()
	defer packageState.mu.RUnlock()
	return packageState.lastCapture
}

// resetForTesting clears package-level state between unit tests.
// Package-private — only callable from test files in the same package.
func resetForTesting() {
	packageState.ready.Store(false)
	packageState.shutdownFlag.Store(false)
	packageState.mu.Lock()
	packageState.lastCapture = ""
	packageState.mu.Unlock()
}
