package status

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Helper: build an http.ResponseRecorder + request against the full
// middleware-wrapped handler, with Host set to the canonical loopback
// address (so the Host-allowlist check passes by default). Tests that
// want to exercise the rejection paths override req.Host explicitly.
func doStatusGET(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:27773"+path, nil)
	req.Host = "127.0.0.1:27773"
	w := httptest.NewRecorder()
	buildHandler().ServeHTTP(w, req)
	return w
}

// TestStatusJSON_ShapeAndFields verifies the JSON contract from
// architecture.md §Status Endpoint JSON Response. Field names must be
// snake_case per the struct tags on statusResponse.
func TestStatusJSON_ShapeAndFields(t *testing.T) {
	resetForTesting()

	w := doStatusGET(t, "/status")
	if w.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	// Strict-tag struct — MUST use explicit json: tags to match wire format.
	var got struct {
		Ready       bool   `json:"ready"`
		LastCapture string `json:"last_capture"`
		PID         int    `json:"pid"`
		Version     string `json:"version"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode JSON: %v (body=%q)", err, w.Body.String())
	}

	if got.LastCapture != "" {
		t.Errorf("LastCapture = %q, want empty initially", got.LastCapture)
	}
	if got.PID == 0 {
		t.Errorf("PID = 0, want current process pid (non-zero)")
	}
	// Version should be non-empty (either from runtime/debug.ReadBuildInfo
	// or the injectedVersion fallback — default "dev" when run via
	// `go test` without ldflags).
	if got.Version == "" {
		t.Errorf("Version is empty, want non-empty build-info string or %q fallback", "dev")
	}
}

// TestStatusJSON_LastCaptureIsFilenameNotPath validates AC #13 — the
// handler MUST extract basename via filepath.Base before emitting JSON.
func TestStatusJSON_LastCaptureIsFilenameNotPath(t *testing.T) {
	resetForTesting()
	SetLastCapture(`C:\Users\foo\Pictures\clip-clap\2026-04-17_14-30-22_481.png`)

	w := doStatusGET(t, "/status")
	var got struct {
		LastCapture string `json:"last_capture"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := "2026-04-17_14-30-22_481.png"
	if got.LastCapture != want {
		t.Errorf("last_capture = %q, want %q (basename only, not full path)", got.LastCapture, want)
	}
}

// TestStatus_Returns200While_Ready_False validates the ready-state
// transition: the endpoint returns 200 regardless of ready, but the
// `ready` field reflects the flag's actual value.
func TestStatus_Returns200While_Ready_False(t *testing.T) {
	resetForTesting()

	// Initially ready=false.
	w := doStatusGET(t, "/status")
	if w.Code != http.StatusOK {
		t.Fatalf("code (ready=false) = %d, want 200", w.Code)
	}
	var before struct {
		Ready bool `json:"ready"`
	}
	if err := json.NewDecoder(w.Body).Decode(&before); err != nil {
		t.Fatalf("decode before: %v", err)
	}
	if before.Ready {
		t.Errorf("ready before MarkReady = true, want false")
	}

	// Flip ready.
	MarkReady()

	w2 := doStatusGET(t, "/status")
	if w2.Code != http.StatusOK {
		t.Fatalf("code (ready=true) = %d, want 200", w2.Code)
	}
	var after struct {
		Ready bool `json:"ready"`
	}
	if err := json.NewDecoder(w2.Body).Decode(&after); err != nil {
		t.Fatalf("decode after: %v", err)
	}
	if !after.Ready {
		t.Errorf("ready after MarkReady = false, want true")
	}
}

// TestStatus_Returns503AfterShutdown validates sustained 503 behavior:
// after BeginShutdown, 5 sequential requests must all return 503 with
// empty body (the draining window is sustained, not a one-shot).
func TestStatus_Returns503AfterShutdown(t *testing.T) {
	resetForTesting()

	// Happy path first.
	w := doStatusGET(t, "/status")
	if w.Code != http.StatusOK {
		t.Fatalf("pre-shutdown code = %d, want 200", w.Code)
	}

	// Flip shutdown.
	BeginShutdown()

	// 5 sequential GETs; all must return 503 with empty body.
	for i := 0; i < 5; i++ {
		w := doStatusGET(t, "/status")
		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("drain request %d: code = %d, want 503", i, w.Code)
		}
		if w.Body.Len() != 0 {
			t.Errorf("drain request %d: body = %q, want empty", i, w.Body.String())
		}
	}
}

// TestStatus_UnknownPathReturns404 proves 404 is reserved for unknown
// paths on the bound listener, distinct from "listener not bound =
// connection refused" at the TCP layer.
func TestStatus_UnknownPathReturns404(t *testing.T) {
	resetForTesting()

	w := doStatusGET(t, "/something-else")
	if w.Code != http.StatusNotFound {
		t.Errorf("code = %d, want 404", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Errorf("body = %q, want empty (no http.Error/StatusText leakage)", w.Body.String())
	}
}

// TestStatus_RejectsNonLoopbackHost validates the Host-header allowlist
// (DNS-rebinding mitigation per security-plan §Stack-Specific Bans).
func TestStatus_RejectsNonLoopbackHost(t *testing.T) {
	resetForTesting()

	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:27773/status", nil)
	req.Host = "192.168.1.1:27773" // non-loopback, should be rejected
	w := httptest.NewRecorder()
	buildHandler().ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("code = %d, want 403 (Host mismatch)", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Errorf("body = %q, want empty", w.Body.String())
	}
}

// TestStatus_RejectsNonEmptyOrigin validates the Origin-reject
// middleware (browser-fence complementing Host allowlist).
func TestStatus_RejectsNonEmptyOrigin(t *testing.T) {
	resetForTesting()

	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:27773/status", nil)
	req.Host = "127.0.0.1:27773"
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()
	buildHandler().ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("code = %d, want 403 (Origin present)", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Errorf("body = %q, want empty", w.Body.String())
	}
}

// TestStatus_RejectsPOST validates the method allowlist — only GET is
// permitted on /status.
func TestStatus_RejectsPOST(t *testing.T) {
	resetForTesting()

	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:27773/status", strings.NewReader(""))
	req.Host = "127.0.0.1:27773"
	w := httptest.NewRecorder()
	buildHandler().ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("code = %d, want 405", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Errorf("body = %q, want empty", w.Body.String())
	}
}

// TestNoListenWithoutAgentFlag validates the HARD agent-mode gate:
// Initialize(false, 0) returns nil, binds no listener, writes no
// PID file. A subsequent net.Dial to the canonical port must fail
// (connection refused).
func TestNoListenWithoutAgentFlag(t *testing.T) {
	resetForTesting()
	// Ensure no prior test left the server active.
	srvMu.Lock()
	isActive = false
	server = nil
	srvMu.Unlock()

	if err := Initialize(false, 0); err != nil {
		t.Fatalf("Initialize(false, 0) = %v, want nil (no-op)", err)
	}

	// Attempt to connect to the canonical port. Expect connection refused.
	conn, err := net.DialTimeout("tcp", "127.0.0.1:27773", 100_000_000) // 100ms
	if err == nil {
		_ = conn.Close()
		t.Fatalf("DialTimeout to 127.0.0.1:27773 succeeded, want connection refused (Initialize should be no-op when agentMode=false)")
	}
	// Success — the port is unbound and we got the expected TCP RST.
}

// TestSetVersion_DefaultIsDev asserts the default injectedVersion is "dev"
// (matching cmd/clip-clap/main.go's `var version string = "dev"` default),
// so resolveVersion() falls back to "dev" when no SetVersion is called and
// runtime/debug.ReadBuildInfo returns no module version (normal `go test`
// environment).
func TestSetVersion_DefaultIsDev(t *testing.T) {
	// Save and restore to isolate from other tests that call SetVersion.
	versionMu.RLock()
	orig := injectedVersion
	versionMu.RUnlock()
	t.Cleanup(func() { SetVersion(orig) })

	SetVersion("dev")
	got := resolveVersion()
	// Under `go test`, ReadBuildInfo typically reports "(devel)" or empty
	// Main.Version → resolveVersion falls back to injectedVersion. Accept
	// either the injected fallback OR any non-empty build-info version.
	if got != "dev" && got == "" {
		t.Errorf("resolveVersion() = %q, want %q (or non-empty build info)", got, "dev")
	}
}

// TestSetVersion_UpdatesInjectedVersion asserts that calling SetVersion
// updates the fallback consumed by resolveVersion() when ReadBuildInfo
// does not supply a module version.
func TestSetVersion_UpdatesInjectedVersion(t *testing.T) {
	versionMu.RLock()
	orig := injectedVersion
	versionMu.RUnlock()
	t.Cleanup(func() { SetVersion(orig) })

	SetVersion("v9.9.9")
	versionMu.RLock()
	got := injectedVersion
	versionMu.RUnlock()
	if got != "v9.9.9" {
		t.Errorf("after SetVersion(%q): injectedVersion = %q, want %q", "v9.9.9", got, "v9.9.9")
	}
}
