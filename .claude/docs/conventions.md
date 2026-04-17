# Project Conventions

_Extracted from `.andromeda/architecture.md` Conventions section by `/setup-project`. Reference for detailed conventions that don't fit in CLAUDE.md's 200-line budget._

## File & directory naming
- **Go source files:** `snake_case` for multi-word `.go` files (e.g., `layered_window.go`, `win32_helpers.go`); single-word lowercase for one-word names
- **Go packages:** single-word lowercase (`tray`, `overlay`, `hotkey`, `capture`, `clipboard`, `toast`, `config`, `log`, `status`)
- **Avoid `clipclap` as a package prefix** — it's the module name; use the short package name instead
- **Scripts and PowerShell:** `kebab-case` (`agent-run.ps1`)
- **Manifests / config templates:** `kebab-case` (`app.manifest`, `goversioninfo.json`)
- **Test files:** Go tests colocated as `{name}_test.go`; Python tests under `tests/integration/test_*.py`

## Identifier naming (Go)
- **Exported types and functions:** `PascalCase` (e.g., `Overlay`, `HandleCapture`)
- **Unexported / locals:** `camelCase`
- **`ALL_CAPS`** is reserved for Win32 constant passthroughs (`user32.CF_UNICODETEXT`) — do NOT use for general Go constants (use `PascalCase` for exported, `camelCase` for unexported)
- **Receiver names:** short, 1–3 letters (`func (s *Service) DoThing()`, not `func (service *Service)`)
- **Avoid shadowing** outer `err`, `ctx`, `db` — common bug source; the linter catches this when run

## Data model conventions
- **`capture_id`** is a ULID (string, sortable, collision-free) — see `architecture.md` `[ID Strategy]`
- **Timestamps in log events:** RFC 3339 nanosecond precision (e.g., `2026-04-16T21:09:52.123456789Z`)
- **Absolute Windows paths** preserved verbatim (e.g., `C:\Users\turbolet85\Pictures\clip-clap\2026-04-16_21-09-52_001.png`); auto-quoted with double quotes if path contains spaces
- **No UUID v4, no Unix timestamps, no relative paths** in user-facing output
- **Filename timestamp format:** `YYYY-MM-DD_HH-MM-SS_mmm.png` (3-digit zero-padded millisecond suffix, local system timezone). Hardcoded; no template engine in v1

## Config file format (TOML)
- **Location:** `%APPDATA%\clip-clap\config.toml` (overridable via `CLIP_CLAP_CONFIG` env var)
- **Keys:**
  - `save_folder` — absolute Windows path
  - `hotkey` — string (default `"Ctrl+Shift+S"`)
  - `auto_quote_paths` — boolean (default `true`)
  - `log_level` — `"INFO"` or `"DEBUG"`
- **Strict unmarshal mode** — unknown keys are rejected at parse time; emits `config.error` event
- All keys optional; defaults provided on first run

## API contracts
- **N/A for users** (no product API, no REST, no gRPC, no versioning)
- **Internal status endpoint** (test hook only): `GET http://127.0.0.1:27773/status` returns flat JSON `{"ready": bool, "last_capture": "...", "pid": int, "version": "..."}` — see `architecture.md` `[Standard Contracts]`

## Error handling (Go)
- Wrap errors with context: `fmt.Errorf("context: %w", err)` — preserves chain for `errors.Is`/`errors.As`
- Each subsystem logs fatal errors via `slog` with `event=<subsystem>.error` before bubbling up
- Tray menu surfaces "Last error" as a grayed-out entry when non-nil
- **No panic-recover except at goroutine boundaries** of each subsystem (tray pump, overlay pump, hotkey pump, status HTTP server)
- See `.claude/rules/observability.md` for log event enum

## Logging
See `.claude/rules/observability.md` for logging rules that apply when editing logger-related files. This doc lists *conventions*; the rule file is *enforcement*.

## Testing
See `.claude/rules/testing.md` for testing rules. This doc describes *conventions* (test naming, structure); the rule file has *enforcement* for path-matched files.

## Imports / module organization (Go)
- Standard library imports first, then third-party, then internal — separated by blank lines (gofmt enforces this when run with `goimports`)
- Use full module paths for internal packages (`github.com/Turbolet85/clip-clap/internal/capture`), not relative imports
- Avoid `init()` for complex setup — use explicit constructors. `init()` is ordering-sensitive and error-prone

## Cross-references
- For complete architecture, see `.andromeda/architecture.md`
- For directory layout, see `.andromeda/architecture.md` Infrastructure Patterns section
- For warnings and gotchas, see `.claude/docs/gotchas.md`
- For runtime-discovered learnings, see `.claude/docs/session-learnings.md`
