// Package main is the clip-clap entry point.
//
// The version flag is processed via the run() seam (testable from
// main_test.go) before any subsystem is initialized.
//
// Phase 2 extends the seam with:
//   - Win32 message-only window + WndProc callback (replaces SIGINT
//     signal block) so the tray menu's Quit can cleanly close the app
//   - tray.RegisterIcon() — deep-ink aperture in the notification area
//   - hotkey.Register() — Ctrl+Shift+S (or config override) bound to WM_HOTKEY
//   - GetMessage/DispatchMessage pump that runs until WM_CLOSE → WM_QUIT
//
// The goversioninfo directive below produces cmd/clip-clap/resource.syso
// at `go generate` time. Paths are relative to this file's directory
// (cmd/clip-clap/), so they reach back two levels to the project root
// where assets/ and goversioninfo.json live.
//
//go:generate goversioninfo -64 -icon=../../assets/app.ico -manifest=../../assets/app.manifest ../../goversioninfo.json
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"image"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/Turbolet85/clip-clap/internal/capture"
	"github.com/Turbolet85/clip-clap/internal/clipboard"
	"github.com/Turbolet85/clip-clap/internal/config"
	"github.com/Turbolet85/clip-clap/internal/hotkey"
	"github.com/Turbolet85/clip-clap/internal/lasterror"
	"github.com/Turbolet85/clip-clap/internal/logger"
	"github.com/Turbolet85/clip-clap/internal/overlay"
	"github.com/Turbolet85/clip-clap/internal/status"
	"github.com/Turbolet85/clip-clap/internal/toast"
	"github.com/Turbolet85/clip-clap/internal/tray"
)

// version is overridable via -X main.version=<value> at build time; falls back to "dev" if unset. The ldflag value MUST include the "v" prefix (e.g., "v1.0.0") to match the release tag format.
var version string = "dev"

// messageClassName is the Win32 window class registered once per process.
// Unique enough that collisions are implausible, but scoped to a single
// atom so UnregisterClassW on teardown releases it cleanly. Must match
// the string passed to CreateWindowExW.
const messageClassName = "ClipClapMessageWindow"

// Package-level state shared between run() (startup wiring) and wndProc
// (window-message dispatch). wndProc must be a package-level named
// function (per design-system Step 8 — `windows.NewCallback` requires a
// stable address for the callback's lifetime); it cannot be a closure
// capturing run()'s locals, so these vars are the coupling point.
var (
	// mainHwnd holds the HWND of the message-only window created in
	// runMessagePump. wndProc reads it for PostMessage(WM_CLOSE) and
	// tray.ShowContextMenu dispatch.
	mainHwnd uintptr

	// mainCfg is the Phase 1 Config loaded at startup. wndProc's menu
	// handlers (Open folder) need cfg.SaveFolder; set once in run() and
	// never mutated after the pump starts.
	mainCfg *config.Config

	// mutexErr holds the result of CreateMutex between the mutex-check
	// in Phase 1's Step 11 and the event-emission in Step 12 (cannot
	// emit the slog event until logger is initialized, so the error is
	// deferred via this package variable). Package-level scope required;
	// a function-local variable would not be reachable from Step 12.
	mutexErr error

	// wndProcCallback is the syscall.NewCallback-wrapped WndProc. Must
	// be set ONCE at program start (lazily, on first pump-run) because
	// syscall.NewCallback allocates a trampoline with a stable address
	// for the callback's lifetime. Re-wrapping in multiple pump-runs
	// would leak trampolines.
	wndProcCallback uintptr

	// unkillableHookActive gates the WM_CLOSE-swallowing hook used by
	// the test harness's taskkill-fallback exercise. Double-gated: the
	// --unkillable-debug flag must be parseable (debug build tag) AND
	// `CLIP_CLAP_TEST_UNKILLABLE=1` env var must be set before this
	// atomic flips to true. Release builds never see the flag, so the
	// hook is physically unreachable on production binaries.
	unkillableHookActive atomic.Bool
)

// waitForShutdown is the hookable shutdown entry point. In production
// (default), it runs the full Win32 message pump: create class → create
// window → register tray icon → register hotkey → GetMessage loop until
// WM_QUIT → teardown. Tests override this to a no-op so run() returns
// immediately after the subsystem-init phase (Phase 1 behavior preserved).
//
// Phase 2 note: this replaces the Phase 1 SIGINT/SIGTERM signal block.
// The production binary is built with `-H windowsgui`, which detaches
// the console — Ctrl+C has no path to reach the process. The tray menu
// "Quit" is the canonical shutdown path; the WM_CLOSE it posts unwinds
// the pump and teardown follows.
var waitForShutdown = runMessagePump

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
	agentModeFlag := fs.Bool("agent-mode", false, "enable HTTP status endpoint on 127.0.0.1:27773 (test-only)")

	// --unkillable-debug is only registered on debug builds. On release
	// builds, unkillableDebugEnabled is false (from unkillable_release.go),
	// so fs.Bool is never called and passing --unkillable-debug triggers
	// "flag provided but not defined" — the security-plan §Input Validation
	// enforcement boundary.
	var unkillableDebugFlag *bool
	if unkillableDebugEnabled {
		unkillableDebugFlag = fs.Bool(
			"unkillable-debug",
			false,
			"register WM_CLOSE handler that refuses to close (test only; requires CLIP_CLAP_TEST_UNKILLABLE=1)",
		)
	}

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
		fmt.Fprintf(stdout, "clip-clap %s\n", version)
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
	mainCfg = cfg

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

	// Phase 4: read CLIP_CLAP_TEST_READY_DELAY_MS env var (test-only
	// escape hatch for exercising the false→true ready-flag edge).
	// Absent → 0. Unparseable → slog.Warn with EventConfigError and
	// treat as 0 (non-fatal; production behavior preserved). This uses
	// EventConfigError, NOT EventAgentDisabled — the failure is a
	// config-parse error, not an agent-mode state decision.
	readyDelayMs := 0
	if raw := os.Getenv("CLIP_CLAP_TEST_READY_DELAY_MS"); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil {
			// Redact the raw value to at most 32 chars to avoid log-
			// flooding if the user set a huge string.
			shortRaw := raw
			if len(shortRaw) > 32 {
				shortRaw = shortRaw[:32] + "…"
			}
			slog.Warn("CLIP_CLAP_TEST_READY_DELAY_MS parse failed",
				"event", logger.EventConfigError,
				"reason", "CLIP_CLAP_TEST_READY_DELAY_MS parse failed",
				"value", shortRaw)
		} else {
			readyDelayMs = parsed
		}
	}

	// Phase 4: activate the unkillable-debug hook iff BOTH the flag is
	// true AND the env var is set. Double-gate per security-plan §Input
	// Validation — either alone must not activate the hook.
	if unkillableDebugFlag != nil && *unkillableDebugFlag &&
		os.Getenv("CLIP_CLAP_TEST_UNKILLABLE") == "1" {
		unkillableHookActive.Store(true)
	}

	// Phase 5: inject the ldflag-driven version into the status package
	// BEFORE Initialize() spawns any goroutines that might read it. The
	// SetVersion call uses a sync.RWMutex internally so a subsequent
	// concurrent read from the HTTP handler is race-safe.
	status.SetVersion(version)

	// Phase 4: spawn the status.Initialize goroutine BEFORE invoking
	// the blocking message pump. Running concurrently means the HTTP
	// listener binds and the PID file is written while the pump
	// processes hotkey/tray/overlay messages. If --agent-mode is
	// false, Initialize is a no-op (returns nil without binding).
	go func() {
		if err := status.Initialize(*agentModeFlag, time.Duration(readyDelayMs)*time.Millisecond); err != nil {
			slog.Error("status server init failed",
				"event", logger.EventHotKeyError,
				"error", err.Error())
		}
	}()

	// Phase 2: message pump (or test override). In production this creates
	// the message-only window, registers the tray icon, binds the hotkey,
	// and blocks on GetMessage until WM_QUIT is posted. Tests override
	// waitForShutdown to return immediately.
	waitForShutdown()
	return 0
}

// runMessagePump is the production implementation of waitForShutdown. It
// owns the full Win32 UI lifecycle: register window class → create
// message-only window → register tray icon → register global hotkey →
// pump messages until WM_QUIT → teardown in reverse order.
//
// Any Win32 failure before the pump starts is logged and returns early;
// the process's exit code is driven by run(), which has already committed
// to 0 by the time waitForShutdown is invoked. Logging the error to the
// "Last error" slot is sufficient visibility — the user can open the log
// file or see the error through the tray menu (if the tray itself came up).
//
// AC #5 ordering is preserved by the subsystem init sequence here:
// (1) window → (2) tray.RegisterIcon (silent) → (3) hotkey.Register
// (emits EventHotKeyRegistered) → (4) GetMessage loop. No other slog
// record fires between config.loaded and hotkey.registered.
func runMessagePump() {
	// Install the WndProc callback once per process. windows.NewCallback
	// allocates a trampoline that must live for the entire pump's lifetime;
	// re-wrapping on every call would leak. Use the package-level atom.
	if wndProcCallback == 0 {
		wndProcCallback = windows.NewCallback(wndProc)
	}

	classNamePtr, err := windows.UTF16PtrFromString(messageClassName)
	if err != nil {
		slog.Error("UTF16PtrFromString(className) failed",
			"event", logger.EventHotKeyError,
			"error", err.Error())
		return
	}
	hInstance, _, _ := tray.GetModuleHandle()
	wc := wndClassExW{
		CbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
		Style:         CS_HREDRAW | CS_VREDRAW,
		LpfnWndProc:   wndProcCallback,
		HInstance:     hInstance,
		LpszClassName: classNamePtr,
	}
	classAtom, _, regErr := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	if classAtom == 0 {
		slog.Error("RegisterClassExW failed",
			"event", logger.EventHotKeyError,
			"error", fmt.Sprintf("%v", regErr))
		return
	}
	defer procUnregisterClassW.Call(uintptr(unsafe.Pointer(classNamePtr)), hInstance)

	// CreateWindowExW signature:
	//   HWND CreateWindowExW(DWORD dwExStyle, LPCWSTR lpClassName,
	//     LPCWSTR lpWindowName, DWORD dwStyle, int X, int Y, int nWidth,
	//     int nHeight, HWND hWndParent, HMENU hMenu, HINSTANCE hInstance,
	//     LPVOID lpParam);
	// Parent = HWND_MESSAGE makes it a hidden message-only window.
	hwnd, _, createErr := procCreateWindowExW.Call(
		uintptr(WS_EX_NOACTIVATE),
		uintptr(unsafe.Pointer(classNamePtr)),
		0,
		0,
		0, 0, 0, 0,
		HWND_MESSAGE,
		0,
		hInstance,
		0,
	)
	if hwnd == 0 {
		slog.Error("CreateWindowExW failed",
			"event", logger.EventHotKeyError,
			"error", fmt.Sprintf("%v", createErr))
		return
	}
	defer procDestroyWindow.Call(hwnd)
	mainHwnd = hwnd

	// Register tray icon. Phase 2 emits NO slog event on success/failure
	// here — the main.go wrapper handles visibility for startup ordering
	// (AC #5: hotkey.registered is the first event after config.loaded).
	if err := tray.RegisterIcon(hwnd); err != nil {
		sanitized := tray.SanitizeForTray(err)
		lasterror.Set(sanitized)
		slog.Error("tray registration failed",
			"event", logger.EventHotKeyError,
			"error", sanitized.Error())
		// F19 tolerance: continue running without a tray icon. The hotkey
		// may still work and the user can kill the process via Task
		// Manager if needed.
	}
	defer func() {
		if err := tray.UnregisterIcon(hwnd); err != nil {
			slog.Error("tray unregister failed",
				"event", logger.EventHotKeyError,
				"error", err.Error())
		}
	}()

	// Register the configured hotkey. hotkey.Register emits its own
	// EventHotKeyRegistered / EventHotKeyError slog records; we do not
	// log here to preserve AC #5's event ordering.
	if mainCfg != nil {
		_ = hotkey.Register(hwnd, mainCfg.Hotkey, 1)
		// F19 tolerance: on registration failure, Register logged the
		// error and returned non-nil; we continue running so the tray
		// icon and menu remain usable even without an active hotkey.
	}

	// Register the AppUserModelID for Windows toast notifications. Done
	// at startup (idempotent) rather than on first capture to avoid the
	// latency hit when the user fires their first Ctrl+Shift+S.
	if err := toast.RegisterAppUserModelID(toast.DefaultAppID); err != nil {
		// Non-fatal — the app runs without toasts; flash + tooltip
		// remain as primary feedback per design-system.
		sanitized := tray.SanitizeForTray(err)
		lasterror.Set(sanitized)
		slog.Error("toast AppUserModelID registration failed",
			"event", logger.EventToastError,
			"error", sanitized.Error())
	}

	// Wire the tray menu handlers to their implementations. The handlers
	// must be set BEFORE the pump starts so the first right-click gets
	// correct behavior. SetHandlers accepts function pointers — no import
	// cycle between tray and clipboard.
	tray.SetHandlers(
		func() { runCaptureFlow(hwnd) },
		func() { runUndoFlow(hwnd) },
		clipboard.HasSnapshot,
	)

	// Start the background "Last error" updater (Phase 2 stub; Phase 3
	// may expand). Safe to call — internal goroutine exits when the
	// process dies.
	tray.UpdateLastErrorMenu(hwnd)

	// GetMessage pump. Per Win32 contract:
	//   ret > 0 → normal message, dispatch
	//   ret == 0 → WM_QUIT, exit the loop
	//   ret < 0 → error; golang.org/x/sys lazy-proc Call returns this
	//     as ret = ^uintptr(0) (i.e., -1) and a non-nil err via
	//     GetLastError. We treat the non-zero-error case as fatal.
	var m msg
	for {
		ret, _, getErr := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		// When ret has all bits set (int32 -1), that's GetMessage
		// returning WinErr per the above contract. Also check getErr
		// which lazy-proc Call populates from GetLastError.
		if int32(ret) == -1 {
			slog.Error("message pump failed",
				"event", logger.EventHotKeyError,
				"error", fmt.Sprintf("GetMessageW returned -1: %v", getErr))
			return
		}
		if ret == 0 {
			// WM_QUIT received — clean exit.
			return
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

// wndProc is the Win32 window procedure for clip-clap's message-only
// window. MUST be a package-level named function (not a closure) so
// windows.NewCallback produces a stable trampoline address for the
// lifetime of the pump. Any panic inside the dispatch is recovered,
// sanitized, and re-panicked to the main goroutine so crashes don't
// silently corrupt the message queue.
func wndProc(hwnd, msgCode, wparam, lparam uintptr) (ret uintptr) {
	defer func() {
		if r := recover(); r != nil {
			var err error
			switch v := r.(type) {
			case error:
				err = v
			case string:
				err = errors.New(v)
			default:
				err = fmt.Errorf("%v", v)
			}
			sanitized := tray.SanitizeForTray(err)
			lasterror.Set(sanitized)
			slog.Error("wndProc panicked",
				"event", logger.EventHotKeyError,
				"error", sanitized.Error())
			// Re-panic to the main goroutine so a corrupted pump surfaces
			// loudly rather than silently swallowing the failure.
			panic(r)
		}
	}()

	switch uint32(msgCode) {
	case tray.WM_HOTKEY:
		// WM_HOTKEY fires when the user presses the registered hotkey
		// (wparam==1 is our hotkey ID). Spawn the capture flow on a
		// background goroutine so the WndProc dispatch returns quickly
		// — overlay.CreateOverlay must NOT block the message pump; the
		// overlay uses its own WndProc running on the pump's thread via
		// Windows dispatch.
		if wparam == 1 {
			go runCaptureFlow(hwnd)
		}
		return 0
	case tray.WM_COMMAND:
		// Low 16 bits of wparam carry the menu item id per Win32 docs.
		id := int(wparam & 0xFFFF)
		tray.HandleMenuCommand(hwnd, id, mainCfg)
		return 0
	case tray.TrayCallback:
		// Tray icon callback — lparam's low 16 bits carry the
		// Win32 mouse-message value (WM_RBUTTONUP on right-click).
		if uint32(lparam)&0xFFFF == tray.WM_RBUTTONUP {
			tray.ShowContextMenu(hwnd)
		}
		return 0
	case tray.WM_CLOSE:
		// Phase 4 unkillable-debug hook: if the double-gated test flag
		// is active (debug build + CLIP_CLAP_TEST_UNKILLABLE=1), swallow
		// WM_CLOSE without posting WM_QUIT — simulates a hung process
		// so the test harness can exercise the taskkill fallback path
		// in scripts/agent-run.ps1 (test_agent_run_ps1_kill_falls_back_to_taskkill).
		if unkillableHookActive.Load() {
			return 0
		}

		// Phase 4: gracefully shut down the status server before
		// posting WM_QUIT. Shutdown() is idempotent (safe no-op when
		// --agent-mode wasn't set) and bounded by a 2s deadline so
		// slow in-flight requests don't freeze the message pump.
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = status.Shutdown(ctx)

		// Graceful shutdown — post WM_QUIT so the GetMessage loop in
		// runMessagePump returns 0 and we can run cleanup defers.
		procPostQuitMessage.Call(0)
		return 0
	default:
		r, _, _ := procDefWindowProcW.Call(hwnd, msgCode, wparam, lparam)
		return r
	}
}

// runCaptureFlow is invoked on every WM_HOTKEY press AND on every "Expose"
// menu-item click. Spawns the transparent overlay, waits for the user to
// drag a rectangle (or press Esc), runs the full capture pipeline end-to-end,
// and fires the 350ms safelight flash on success.
//
// All errors are non-fatal — they set `lasterror` via SanitizeForTray
// (strips absolute paths) and are surfaced via the tray's "Last error"
// menu slot. The message pump MUST NOT die on capture failures.
func runCaptureFlow(hwnd uintptr) {
	if mainCfg == nil {
		return
	}
	err := overlay.CreateOverlay(func(rect image.Rectangle) {
		// Run the pipeline: capture → clipboard → toast → tooltip → flash.
		captureID, absPath, cerr := capture.Capture(rect, mainCfg.SaveFolder, time.Now)
		if cerr != nil {
			// capture.Capture already emitted capture.failed + set lasterror
			// via SanitizeForTray. Nothing more to do.
			return
		}

		// Clipboard swap with auto-quote per cfg.AutoQuotePaths.
		clipText := clipboard.Quote(absPath, mainCfg.AutoQuotePaths)
		if err := clipboard.Swap(clipText, captureID); err != nil {
			sanitized := tray.SanitizeForTray(err)
			lasterror.Set(sanitized)
			slog.Error("clipboard swap failed",
				"event", logger.EventCaptureFailed,
				"capture_id", captureID,
				"error", sanitized.Error())
			return
		}

		// Toast notification (non-fatal — tray flash is primary receipt).
		_ = toast.Show(absPath, captureID, mainCfg.SaveFolder)

		// Update tray tooltip to "Last: <filename>".
		filename := filepath.Base(absPath)
		_ = tray.UpdateTooltipAfterCapture(hwnd, filename)

		// Fire the signature 350ms safelight-amber flash.
		_ = tray.Flash(hwnd)
	})
	if err != nil {
		sanitized := tray.SanitizeForTray(err)
		lasterror.Set(sanitized)
		slog.Error("overlay create failed",
			"event", logger.EventCaptureFailed,
			"error", sanitized.Error())
	}
}

// runUndoFlow restores the prior clipboard contents (via clipboard.Undo)
// and reverts the tray tooltip to its idle state. Clears lasterror so the
// next right-click shows "Last error: <none>".
func runUndoFlow(hwnd uintptr) {
	if err := clipboard.Undo(); err != nil {
		sanitized := tray.SanitizeForTray(err)
		lasterror.Set(sanitized)
		slog.Error("clipboard undo failed",
			"event", logger.EventClipboardUndo,
			"error", sanitized.Error())
		return
	}
	_ = tray.RevertTooltip(hwnd)
	lasterror.Set(nil) // clear the "Last error" display
}

// main wraps run() so os.Exit only fires outside test contexts. syscall
// reference kept in imports for the windows package's transitive usage.
func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

// _ keeps syscall in the import list for any future signal handling we
// may add (Phase 3+ status-endpoint shutdown may reintroduce it). The
// blank ref avoids the unused-import compile error while signaling intent.
var _ = syscall.SIGINT
