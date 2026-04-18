# Changelog

All notable changes to clip-clap are documented here.
Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
This project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] — 2026-04-18

The first public release of clip-clap. Windows 10/11 tray utility:
hotkey-triggered area-select screenshot → PNG save → clipboard replacement
with auto-quoted absolute path.

### Added

- **F1 — Single-binary distribution.** Static `clip-clap.exe`
  (`CGO_ENABLED=0`, `-H windowsgui`, `-s -w`). No installer, no admin,
  no dependencies.
- **F2 — Global hotkey.** Default `Ctrl+Shift+S`, registered via direct
  Win32 `RegisterHotKey` on the shared message pump. Configurable via
  TOML.
- **F3 — Area-select overlay.** Full virtual-screen transparent layered
  window (`WS_EX_LAYERED` + `UpdateLayeredWindow`, 32-bpp premultiplied
  alpha); click-and-drag rectangle selection with static contact-sheet
  stroke + safelight-amber edge tick-marks; Esc cancels with a
  symmetric 80 ms fade-out.
- **F4 — Clipboard path replacement with auto-quoting.** Absolute Windows
  path written via `CF_UNICODETEXT`; paths containing spaces are
  double-quoted. Prior clipboard content snapshotted in memory for
  Undo.
- **F5 — Toast notification on capture.** Windows toast via
  `go-toast/toast` with AppUserModelID `Turbolet85.ClipClap` registered
  on first run.
- **F6 — Tray icon + context menu.** `Expose` (capture), Open folder,
  Undo last capture, `Settings (edit config.toml)` (placeholder for F37),
  Last error, Quit. Icon flashes safelight-amber for 350 ms on each
  successful capture.
- **F7 — Undo last capture.** Tray menu entry restores the prior
  clipboard content (in-memory snapshot) without re-fetching the saved
  file.
- **F8 — TOML config file.** `%APPDATA%\clip-clap\config.toml`
  (auto-created on first run) with `save_folder`, `hotkey`,
  `auto_quote_paths`, `log_level`. Strict-mode parser via
  `pelletier/go-toml/v2`; unknown keys rejected.
- **F9 — Structured JSON logging.** `log/slog` JSONHandler to
  `logs/agent-latest.jsonl`; event enum includes `capture.started|
  completed|failed`, `clipboard.swap|undo`, `toast.shown|error`,
  `hotkey.registered|error`, `tray.menu.opened`, `tray.flash.error`,
  `config.loaded|error`, `single_instance.violation`, `agent.disabled`.
- **F10 — Status endpoint for test harness.** `GET
  http://127.0.0.1:27773/status` returns JSON
  (`{"ready","last_capture","pid","version"}`), loopback-only,
  `--agent-mode`-gated (disabled by default).
- **F11 — Single-instance guard.** Windows named mutex
  `Local\ClipClapSingleInstance` (per-session namespace per
  security-plan) prevents duplicate processes.
- **F12 — PID file for agent mode.** `.agent-running` written during
  `--agent-mode` runs; deleted on graceful shutdown.
- **F13 — Build-tag-gated `--unkillable-debug` flag.** Test-only hook
  for taskkill-fallback verification; unparseable on release builds
  (file excluded via `//go:build !debug`).
- **F14 — 2 s-bounded WM_CLOSE shutdown.** Message-pump WM_CLOSE
  triggers `status.Shutdown` with a 2 s context timeout, then
  `PostQuitMessage`.
- **F15 — PowerShell verification harness.** `scripts/agent-run.ps1`
  subcommands `build`, `startAgent`, `killAgent`, `status`, `logs`
  (with alias-safe naming to avoid PowerShell built-in collisions).
- **F16 — pytest + pywinauto integration suite.** `tests/integration/`
  with `agent` fixture, `conftest.py` helpers, and test modules for
  agent-mode, capture flow, clipboard, toast, DPI, config, harness.
- **F17 — CI matrix.** GitHub Actions `test-unit` + `test-verification`
  jobs on `windows-latest` across Go 1.22.x, 1.23.x, 1.24.x. SHA-pinned
  actions with `# vX.Y.Z` trailing comments, enforced by an
  in-workflow lint step.
- **F18 — Resource embedding.** `goversioninfo` v1.5.0 produces
  `resource.syso` with `app.ico` + `app.manifest` (Windows 10/11 compat
  GUIDs, per-monitor DPI awareness).
- **F19 — Capture file naming.** `<year>-<month>-<day>_<hh>-<mm>-<ss>_<ms>.png`
  (e.g., `2026-04-18_14-30-22_481.png`) with millisecond precision — no
  collisions possible.
- **F20 — ULID capture IDs.** `oklog/ulid/v2` with
  `ulid.Monotonic(crypto/rand.Reader, 0)` per security-plan.
- **F21 — Error-sink pattern.** `internal/lasterror.Set/Get` surfaces
  subsystem failures as a grayed "Last error" tray menu entry without
  requiring users to open log files.
- **F22 — Clipboard reentry guard.** Per-capture 500 ms async flag
  prevents the app's own clipboard write from re-triggering capture.
- **F23 — Config env-var overrides.** `CLIP_CLAP_CONFIG`,
  `CLIP_CLAP_SAVE_DIR`, `CLIP_CLAP_LOG_PATH`, `CLIP_CLAP_DEBUG` (each
  independent; documented precedence in architecture.md).
- **F24 — Debug build flag.** `--debug` / `CLIP_CLAP_DEBUG=1` bumps
  `slog` level to DEBUG without requiring a rebuild.
- **F25 — Scoop manifest (`bucket/clip-clap.json`).** v3-compatible
  manifest with `version`, `url`, `hash` (top-level + `architecture.64bit`)
  rewritten by the release workflow on every tag push.
- **F26 — Release workflow (`.github/workflows/release.yml`).**
  Tag-push + `workflow_dispatch` triggers; 4 jobs (`test-unit`,
  `test-verification`, `build-release`, `publish-release`);
  `git describe --tags --match='v*'` enforces v-prefix tag format via
  `-X main.version=`; SHA-256 computed and output-shared; Scoop bucket
  manifest rewritten with UTF-8 BOM-less + field-order preservation.
- **F27 — Conditional signing hook.** `signtool sign` step in
  `release.yml` gated on `env.AZURE_TRUSTED_SIGNING_ENDPOINT != ''`.
  v1.0 ships unsigned (secret absent); v1.1+ path ready.
- **F28 — govulncheck in CI.** `golang/govulncheck-action` SHA-pinned
  in `test.yml` with `continue-on-error: true` (warning-only per
  minimal-tier policy).

### Changed

- Initial release; no breaking changes.

### Fixed

- N/A (initial release).

## [Unreleased]

No changes targeted for the next release yet. Items deferred past v1.0
are tracked in the "Backlog / Deferred Features" section of `README.md`
(F29–F37: code signing rollout, Winget, filename templates, `@`-prefix
mode, annotation, OCR, WinRT toast fallback, arch-doc reconciliation,
Settings UI).
