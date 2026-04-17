// Package main is the clip-clap entry point.
//
// The version flag is processed via the run() seam (testable from
// main_test.go) before any subsystem is initialized.
//
// Phase 1 extends the seam with:
//   - --debug flag parsing alongside --version
//   - single-instance mutex (Local\ClipClapSingleInstance per security-plan)
//   - config.Load() with auto-create-on-first-run + strict-mode TOML
//   - logger.Initialize() with RFC 3339 nanosecond timestamps
//   - config.loaded event emission as the first log line
//   - signal.Notify(SIGINT, SIGTERM) block (placeholder; Phase 2 replaces
//     with WM_CLOSE message pump for the -H windowsgui production binary)
//
// The goversioninfo directive below produces cmd/clip-clap/resource.syso
// at `go generate` time. Paths are relative to this file's directory
// (cmd/clip-clap/), so they reach back two levels to the project root
// where assets/ and goversioninfo.json live.
//
//go:generate goversioninfo -64 -icon=../../assets/app.ico -manifest=../../assets/app.manifest ../../goversioninfo.json
package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"

	"github.com/Turbolet85/clip-clap/internal/config"
	"github.com/Turbolet85/clip-clap/internal/logger"
)

const versionString = "clip-clap v0.0.1"

// mutexErr holds the result of CreateMutex between the mutex-check in Step 11
// and the event-emission in Step 12 (cannot emit the slog event until logger
// is initialized, so the error is deferred via this package variable).
// Package-level scope is required; a function-local variable would not be
// reachable from Step 12's emission path.
var mutexErr error

// waitForShutdown blocks until the process receives SIGINT or SIGTERM. Kept
// as a package variable so tests can replace it with a no-op. Production
// runs keep the default behavior; unit tests that exercise run() override
// this to return immediately.
//
// Phase 2 will replace this with a Win32 WM_CLOSE-driven exit path on the
// tray message pump. For now, `go run` and console-attached smoke tests
// terminate via Ctrl+C; the -H windowsgui production binary has no console
// attached and relies on external taskkill/Stop-Process.
var waitForShutdown = func() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}

// run is the testable seam that main() wraps. Tests in main_test.go invoke
// run directly with custom args + stdout buffer so --version can be exercised
// without spawning a subprocess.
//
// We use fs.Bool("version", ...) on a local FlagSet instead of flag.Bool("version", ...)
// on the global FlagSet — the local FlagSet can be re-parsed many times in tests
// without polluting global flag state (the global flag.Bool would panic on the
// second test run with "flag redefined: version").
func run(args []string, stdout io.Writer) int {
	fs := flag.NewFlagSet("clip-clap", flag.ContinueOnError)
	// Writer target for flag package output. Routing to stdout (not io.Discard)
	// means `--help` prints the flag listing to the provided writer instead of
	// being silently swallowed — required by /implement Final 5 scaffolding
	// CLI smoke (`{binary} --help` must return 0 with flag listing).
	fs.SetOutput(stdout)
	versionFlag := fs.Bool("version", false, "print version and exit")
	debugFlag := fs.Bool("debug", false, "enable debug-level logging")
	if err := fs.Parse(args); err != nil {
		// flag.ErrHelp: the user passed -h or --help. flag.Parse already
		// printed the usage to stdout (via SetOutput above) before returning
		// ErrHelp. We return 0 to match standard CLI convention.
		if err == flag.ErrHelp {
			return 0
		}
		// Any other parse error (unknown flag, malformed value) → exit 2.
		// Error message was written to stdout by fs.Parse before returning.
		return 2
	}
	if *versionFlag {
		fmt.Fprintln(stdout, versionString)
		return 0
	}

	// Step 11: single-instance mutex + config load. Mutex first so we fail
	// fast on the most common conflict (another instance running); the
	// violation-event emission is deferred to Step 12 because slog is not
	// yet configured. security-plan §Single-Instance Mutex Namespace
	// mandates Local\ (per-session); NEVER Global\ (cross-session DoS).
	mutexName, _ := windows.UTF16PtrFromString(`Local\ClipClapSingleInstance`)
	var mutexHandle windows.Handle
	mutexHandle, mutexErr = windows.CreateMutex(nil, false, mutexName)

	// Unexpected Win32 errors (e.g., ERROR_ACCESS_DENIED) — fail fast to
	// stderr so the user sees the cause even when stdout is redirected.
	// ERROR_ALREADY_EXISTS is handled below in Step 12 after logger init.
	if mutexErr != nil && mutexErr != windows.ERROR_ALREADY_EXISTS {
		fmt.Fprintf(os.Stderr, "mutex creation failed: %v\n", mutexErr)
		return 1
	}

	cfg, cfgPath, cfgErr := config.Load()
	if cfgErr != nil {
		// Logger isn't initialized yet — emit error to stderr so the user
		// can diagnose without opening the log file (which may not exist).
		// The real config.error slog event is skipped here (logger unavail);
		// the stderr write is the user-facing feedback for this exit path.
		fmt.Fprintf(os.Stderr, "config load failed: %v\n", cfgErr)
		return 1
	}

	// Step 12: resolve log level, initialize logger, emit config.loaded
	// as the FIRST slog call. Logger.Initialize must not emit any records
	// of its own — Step 2 is designed so ReplaceAttr only transforms
	// timestamps, never auto-logs.
	level := slog.LevelInfo
	if *debugFlag || cfg.LogLevel == "DEBUG" {
		level = slog.LevelDebug
	}
	logPath := os.Getenv("CLIP_CLAP_LOG_PATH")
	if logPath == "" {
		logPath = "logs/agent-latest.jsonl"
	}
	if err := logger.Initialize(level, logPath); err != nil {
		fmt.Fprintf(os.Stderr, "logger initialization failed: %v\n", err)
		return 1
	}
	// config_path logged as filepath.Base(cfgPath) per security-plan
	// §Error Handling — avoids leaking full user paths to the log file.
	slog.Info("config loaded",
		"event", logger.EventConfigLoaded,
		"config_path", filepath.Base(cfgPath),
		"save_folder", cfg.SaveFolder,
		"hotkey", cfg.Hotkey)

	// Deferred ERROR_ALREADY_EXISTS handling from Step 11.
	if mutexErr == windows.ERROR_ALREADY_EXISTS {
		// existing_pid = 0 is a Phase 1 placeholder; no Win32 API returns
		// the holder's PID from a named mutex. Phase 2 may extract it via
		// process-enumeration if the cost is justified.
		slog.Error("another instance is already running",
			"event", logger.EventSingleInstanceViolation,
			"existing_pid", 0)
		return 1
	}
	defer windows.CloseHandle(mutexHandle)

	// Step 13: signal blocking (placeholder for Phase 1). Delegated to the
	// package-level waitForShutdown variable so tests can skip the block.
	waitForShutdown()
	return 0
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}
