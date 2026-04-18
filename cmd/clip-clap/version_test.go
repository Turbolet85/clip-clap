// Phase 5 version tests: validate the package-level `version` var default
// and the release-build ldflag override path.
//
// At build time the release CI injects the version via:
//
//	go build -ldflags="-X main.version=v1.0.0" -o clip-clap.exe ./cmd/clip-clap
//
// The default fallback is "dev" so local `go build` (without ldflags) emits
// `clip-clap dev`, not `clip-clap v0.0.1`.

package main

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestVersionVar_DefaultDevFallback asserts the package-level `version`
// variable defaults to "dev" when compiled without `-X main.version=...`.
func TestVersionVar_DefaultDevFallback(t *testing.T) {
	if version != "dev" {
		t.Errorf("expected version=%q (ldflag fallback), got %q", "dev", version)
	}
}

// TestVersionFlag_EmitsOverriddenValue builds a temporary binary with
// `-ldflags=-X main.version=v1.0.0` and asserts the --version flag emits
// the overridden value. This proves the release CI ldflag injection path
// works end-to-end without requiring a tagged build.
func TestVersionFlag_EmitsOverriddenValue(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in -short mode (builds a test binary)")
	}
	tmpDir := t.TempDir()
	binName := "clip-clap-test"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(tmpDir, binName)

	buildCmd := exec.Command("go", "build",
		"-ldflags=-X main.version=v1.0.0",
		"-o", binPath,
		".",
	)
	buildOut, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\noutput: %s", err, buildOut)
	}

	runOut, err := exec.Command(binPath, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("run failed: %v\noutput: %s", err, runOut)
	}

	got := string(runOut)
	// Normalize CRLF to LF for cross-platform assertion robustness.
	got = strings.ReplaceAll(got, "\r\n", "\n")
	const want = "clip-clap v1.0.0\n"
	if got != want {
		t.Errorf("--version output mismatch:\n  want: %q\n  got:  %q", want, got)
	}
}
