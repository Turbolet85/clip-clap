package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Turbolet85/clip-clap/internal/logger"
)

// setupTestEnv isolates run() from the real user environment:
//   - Redirects AppData / XDG_CONFIG_HOME to a TempDir so config.Load's
//     auto-create path writes into a disposable directory.
//   - Redirects CLIP_CLAP_LOG_PATH to a TempDir log file so we can parse
//     the emitted JSON without touching ./logs.
//   - Replaces waitForShutdown with a no-op so run() returns immediately
//     after emitting config.loaded (instead of blocking on SIGINT/SIGTERM).
//   - Closes the logger's file handle at cleanup so Windows can remove
//     the TempDir without "file in use" errors (Cleanup is LIFO, so register
//     Close AFTER any t.TempDir calls).
//
// Returns the log file path for the caller to read.
func setupTestEnv(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("AppData", tmp)
	t.Setenv("XDG_CONFIG_HOME", tmp)
	logPath := filepath.Join(tmp, "logs", "agent-latest.jsonl")
	t.Setenv("CLIP_CLAP_LOG_PATH", logPath)
	// Clear env vars that could interfere with test determinism.
	t.Setenv("CLIP_CLAP_CONFIG", "")
	t.Setenv("CLIP_CLAP_DEBUG", "")
	t.Setenv("CLIP_CLAP_SAVE_DIR", "")

	origWait := waitForShutdown
	waitForShutdown = func() {}
	t.Cleanup(func() {
		waitForShutdown = origWait
		_ = logger.Close()
	})
	return logPath
}

// TestVersionFlag_PrintsLiteral asserts run() with --version writes exactly
// "clip-clap dev\n" to stdout (canonical fmt.Fprintf output using the "dev"
// fallback when no ldflags are applied) and exits 0. --version returns early
// before any subsystem init, so this test does NOT need setupTestEnv.
func TestVersionFlag_PrintsLiteral(t *testing.T) {
	var buf bytes.Buffer
	code := run([]string{"--version"}, &buf)

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	// Without ldflags, version defaults to "dev".
	const want = "clip-clap dev\n"
	if got := buf.String(); got != want {
		t.Errorf("stdout mismatch:\n  want: %q\n  got:  %q", want, got)
	}
}

// TestHelpFlag_ExitsZeroWithUsage asserts run() with --help returns 0 and
// writes a usage block that mentions the -version flag. Required by the
// /implement Final 5 scaffolding CLI smoke contract (`{binary} --help
// returns 0 with expected flag listing`). --help returns early before any
// subsystem init, so this test does NOT need setupTestEnv.
func TestHelpFlag_ExitsZeroWithUsage(t *testing.T) {
	var buf bytes.Buffer
	code := run([]string{"--help"}, &buf)

	if code != 0 {
		t.Errorf("expected --help exit code 0, got %d", code)
	}
	out := buf.String()
	if !strings.Contains(out, "version") {
		t.Errorf("--help output should list the -version flag; got: %q", out)
	}
	// Phase 1 added the --debug flag — verify it appears in the help output.
	if !strings.Contains(out, "debug") {
		t.Errorf("--help output should list the -debug flag (added Phase 1); got: %q", out)
	}
}

// TestDebugFlag_ExitsZero asserts run([]string{"--debug"}) parses the flag,
// completes startup (mutex + config + logger), and returns 0 cleanly (the
// waitForShutdown hook is neutered in setupTestEnv). Step 10 plan's primary
// behavioral test for the --debug flag's happy path.
func TestDebugFlag_ExitsZero(t *testing.T) {
	setupTestEnv(t)

	code := run([]string{"--debug"}, io.Discard)
	if code != 0 {
		t.Errorf("run with --debug should exit 0; got %d", code)
	}
}

// TestDebugFlag_ResolvesToLevelDebug — end-to-end verification that the
// --debug flag causes the logger to emit DEBUG-level records. Runs twice:
// with the flag (asserts DEBUG record present) and without (asserts NO DEBUG
// record). Per Phase 5 validation, this replaces the earlier transitive-
// coverage-only specification.
func TestDebugFlag_ResolvesToLevelDebug(t *testing.T) {
	t.Run("with --debug flag, DEBUG records appear", func(t *testing.T) {
		logPath := setupTestEnv(t)
		if code := run([]string{"--debug"}, io.Discard); code != 0 {
			t.Fatalf("run returned %d", code)
		}
		// Emit a DEBUG record directly via the configured default slog —
		// mirrors the level-gate that future subsystems will experience.
		slog.Debug("debug probe")
		_ = logger.Close()

		if hasDebugRecord(t, logPath) != true {
			t.Errorf("expected at least one DEBUG-level record in log with --debug flag")
		}
	})

	t.Run("without --debug flag, NO DEBUG records appear", func(t *testing.T) {
		logPath := setupTestEnv(t)
		if code := run([]string{}, io.Discard); code != 0 {
			t.Fatalf("run returned %d", code)
		}
		slog.Debug("debug probe")
		_ = logger.Close()

		if hasDebugRecord(t, logPath) != false {
			t.Errorf("expected NO DEBUG-level records in log without --debug flag")
		}
	})
}

// hasDebugRecord parses the log file line-by-line and returns true if any
// record has `"level":"DEBUG"`. Returns false for any error reading / parsing.
func hasDebugRecord(t *testing.T, logPath string) bool {
	t.Helper()
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file %s: %v", logPath, err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue
		}
		if level, _ := record["level"].(string); level == "DEBUG" {
			return true
		}
	}
	return false
}
