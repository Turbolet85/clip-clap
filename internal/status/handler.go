// Package status — loopback-only HTTP /status endpoint, exposed at
// 127.0.0.1:27773 when the binary is launched with --agent-mode.
// This is a TEST HOOK, not a product API (security-plan §Agent-mode gate):
// default off, never exposed externally, never auth-gated, never
// proxied.
//
// Contract (architecture.md §Status Endpoint JSON Response):
//
//	GET /status → 200 with `{"ready":bool,"last_capture":"name","pid":int,"version":"x.y.z"}`
//	GET /status → 503 after BeginShutdown (empty body, draining window)
//	GET /other → 404 (empty body, unknown path)
//	POST /status → 405 (empty body, method not allowed)
//	Host-mismatch → 403 (empty body, DNS-rebinding fence)
//	Origin-present → 403 (empty body, browser CORS fence)
//
// All non-200 responses use `w.WriteHeader(code)` ONLY — we never call
// http.Error or http.StatusText, both of which echo server internals
// into the response body (security-plan §Error Handling).
package status

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"
)

// Compile-time sanity: keep net, time imports alive for Listen / Sleep.
var _ net.Listener

// listenAddr is the hard-coded loopback bind point. Never `0.0.0.0`,
// never `::` — security-plan §Local Endpoint Hardening forbids
// non-loopback bindings.
const listenAddr = "127.0.0.1:27773"

// allowedHosts is the exact set of Host-header values that bypass the
// 403 allowlist middleware. `127.0.0.1:27773` is the canonical form;
// `localhost:27773` covers tools that resolve via /etc/hosts first.
// IPv6 loopback (`[::1]:27773`) is intentionally excluded — we bind
// to IPv4 only (127.0.0.1), so IPv6 clients are already refused at
// the TCP layer.
var allowedHosts = map[string]struct{}{
	"127.0.0.1:27773": {},
	"localhost:27773": {},
}

// versionFallback is returned in the JSON response when
// runtime/debug.ReadBuildInfo() cannot determine the module version
// (e.g., running via `go run` or in tests). Matches the versionString
// in cmd/clip-clap/main.go for consistency.
const versionFallback = "v0.0.1"

// Package-level server state. Initialized once (per process) by
// Initialize; cleared by Shutdown. All mutation is guarded by srvMu.
var (
	srvMu    sync.Mutex
	server   *http.Server
	isActive bool // true once Initialize has bound a listener
)

// Initialize binds the loopback listener and starts the HTTP server
// goroutines. If agentMode is false, returns nil without binding —
// this is the hard agent-mode gate required by the security plan.
//
// Concurrency model (per plan §Step 4):
//   - Bind listener (synchronous, returns error if port is in use).
//   - Spawn goroutine A: srv.Serve(listener) (blocks forever).
//   - Spawn goroutine B: time.Sleep(readyDelay); MarkReady().
//
// The two goroutines run concurrently so the listener accepts
// connections while the delay timer counts down. A single goroutine
// doing Serve() then Sleep() would never reach Sleep — Serve blocks
// forever on the accept loop.
//
// Writes .agent-running via WritePIDFile on successful bind. If the
// PID-file write fails AFTER the listener is bound, we close the
// listener and return the combined error (no partial state).
func Initialize(agentMode bool, readyDelay time.Duration) error {
	if !agentMode {
		// No-op: caller provided no agent mode, no listener should
		// be bound. Subsequent Shutdown() is also a no-op (idempotency
		// contract).
		return nil
	}

	srvMu.Lock()
	defer srvMu.Unlock()

	if isActive {
		// Re-entry guard. Initialize should only be called once per
		// process (from main.go's startup goroutine). Returning nil
		// is safer than panicking — callers can retry harmlessly.
		return nil
	}

	// Bind the loopback listener. This fails fast if another process
	// holds port 27773 (e.g., a stale agent-mode instance).
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}

	// Record PID file for the harness to discover.
	if writeErr := WritePIDFile(".agent-running"); writeErr != nil {
		_ = ln.Close()
		return writeErr
	}

	// Build the HTTP server with routing + middleware stack.
	server = &http.Server{
		Addr:    listenAddr,
		Handler: buildHandler(),
	}
	isActive = true

	// Goroutine A: accept loop. srv.Serve blocks until Close is called.
	go func() {
		// Ignore the "listener closed" error returned by Serve when
		// Shutdown closes it — that's the expected exit path, not a
		// failure. Any other error is also swallowed here because
		// logging belongs to the caller (main.go).
		_ = server.Serve(ln)
	}()

	// Goroutine B: ready-delay timer. Independent of goroutine A so
	// Serve can accept connections while we're still sleeping. This
	// is what the `CLIP_CLAP_TEST_READY_DELAY_MS` integration test
	// exercises to observe the false→true edge.
	go func() {
		if readyDelay > 0 {
			time.Sleep(readyDelay)
		}
		MarkReady()
	}()

	return nil
}

// Shutdown begins the draining window (503 responses) and tears down
// the listener. Idempotent: if Initialize was never called or returned
// without binding, returns nil immediately.
//
// Per plan §Step 4 contract, caller MUST provide a context with a
// deadline. Passing context.Background() would allow server.Shutdown
// to block forever on slow clients, which would freeze the Win32
// message pump in main.go's WM_CLOSE handler.
func Shutdown(ctx context.Context) error {
	srvMu.Lock()
	if !isActive {
		srvMu.Unlock()
		return nil
	}
	srv := server
	// Mark as inactive BEFORE releasing the lock so a racing Initialize
	// (not expected, but defensive) sees inactive state.
	isActive = false
	srvMu.Unlock()

	BeginShutdown()
	err := srv.Shutdown(ctx)
	// Best-effort PID-file cleanup — if it's already gone (PS kill
	// already removed it), DeletePIDFile tolerates the absence.
	_ = DeletePIDFile(".agent-running")
	return err
}

// statusResponse mirrors the JSON contract in architecture.md
// §Status Endpoint JSON Response. Field names use snake_case via
// explicit struct tags — Go's default Marshal would emit PascalCase
// ("Ready", "LastCapture") which tests assert against the wire
// format.
type statusResponse struct {
	Ready       bool   `json:"ready"`
	LastCapture string `json:"last_capture"`
	PID         int    `json:"pid"`
	Version     string `json:"version"`
}

// handleStatus is the /status GET handler. Gated on "not shutting
// down"; returns 503 (empty body) otherwise.
func handleStatus(w http.ResponseWriter, r *http.Request) {
	if IsShutdown() {
		w.WriteHeader(http.StatusServiceUnavailable) // 503
		return
	}

	// filepath.Base on "" returns "." — guard against that by emitting
	// empty string directly when no capture has happened yet.
	var capture string
	if raw := GetLastCapture(); raw != "" {
		capture = filepath.Base(raw)
	}

	resp := statusResponse{
		Ready:       IsReady(),
		LastCapture: capture,
		PID:         processPID(),
		Version:     resolveVersion(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(&resp)
}

// buildHandler constructs the full HTTP handler stack — middleware +
// mux + /status route. Factored out of Initialize so tests can call
// it directly via httptest without binding a real listener.
func buildHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/status", handleStatus)
	return buildMiddleware(mux)
}

// buildMiddleware wraps the mux with the three security-plan-required
// checks, in order:
//  1. Host-header allowlist (DNS-rebinding fence)
//  2. Origin-reject (browser CORS fence)
//  3. Method allowlist (GET only for /status)
//  4. Fallthrough mux (routes to handleStatus or 404).
//
// Ordered this way because:
//   - Origin-rebinding attacks from malicious DNS records arrive with
//     arbitrary Host values → reject at step 1.
//   - Browser-issued cross-origin requests (e.g., a webpage at
//     attacker.com trying to fetch /status) carry an Origin header;
//     API clients (curl, pytest requests) do not → reject at step 2.
//   - Method check last because an unsupported method on an unknown
//     path should prefer 404 (path mismatch) over 405 (method mismatch).
//     ServeMux handles this by matching path first.
func buildMiddleware(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// (1) Host allowlist. DNS-rebinding mitigation per security-plan
		// §Stack-Specific Bans (golang/go#23993). Empty-body 403.
		if _, ok := allowedHosts[r.Host]; !ok {
			w.WriteHeader(http.StatusForbidden) // 403
			return
		}

		// (2) Origin reject. Browser-fence per security-plan §Stack-
		// Specific Bans. API clients don't set Origin; only browser-
		// driven cross-origin requests do.
		if r.Header.Get("Origin") != "" {
			w.WriteHeader(http.StatusForbidden) // 403
			return
		}

		// (3) Method allowlist. Only GET is permitted on /status.
		if r.URL.Path == "/status" && r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed) // 405
			return
		}

		// (4) Route to handler or 404. Our mux only registers /status,
		// so anything else falls through to the custom 404 below
		// (mux.ServeHTTP would use the default NotFoundHandler which
		// writes a body — we want empty body).
		if r.URL.Path != "/status" {
			w.WriteHeader(http.StatusNotFound) // 404
			return
		}

		mux.ServeHTTP(w, r)
	})
}

// resolveVersion returns the module version from build info, or the
// hard-coded fallback if ReadBuildInfo fails (e.g., `go run` without
// a tagged build).
func resolveVersion() string {
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return versionFallback
}

// processPID is a seam for tests — production always returns
// os.Getpid(). A test can substitute a different function pointer
// via testPIDOverride if needed.
var processPID = realPID

// testPIDOverride lets handler_test.go substitute a fixed PID for
// the JSON shape assertion without having to spawn a real process.
// Unused in production.
var testPIDOverride = 0

func realPID() int {
	// inlined to keep the hot path zero-allocation — os.Getpid on
	// Windows is a single syscall.
	return osGetpid()
}

// osGetpid is indirected so Step 1-time unit tests can pin a fixed
// PID. In production it's os.Getpid.
var osGetpid = realOSGetpid

// realOSGetpid is the indirect call target. Declared in a separate
// file for clarity — see handler_osgetpid.go.
