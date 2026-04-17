---
paths:
  - "**/*_test.go"
  - "tests/integration/**"
  - "tests/**/conftest.py"
  - "scripts/agent-run.ps1"
---

# Testing Rules

Path-scoped rules for test files. Loaded only when Claude is working with files matching the `paths:` frontmatter above.

## Frameworks
- **Unit tests:** Go stdlib `testing`, optionally `github.com/stretchr/testify/assert` for ergonomic assertions
- **Integration tests:** Python `pytest` 8.x + `pywinauto` v0.6.8 (UIA backend) — `desktop-windows` verification profile
- **Harness:** `scripts/agent-run.ps1 {build|start|status|logs|kill}` orchestrates build + agent lifecycle for integration tests

## File placement
- **Go unit tests:** colocate with source as `{name}_test.go` in the same package (e.g., `internal/capture/filename_test.go` next to `filename.go`)
- **Python integration tests:** under `tests/integration/test_*.py` with shared fixtures in `tests/integration/conftest.py`
- **No test data in source tree** — generate fixtures programmatically in test bodies

## Unit test patterns (Go)
- Test names follow Go convention: `TestFunctionName_Scenario` (e.g., `TestFormatFilename_WithMillisecondCollision`)
- Table-driven tests for input/output transformations (filename formatter, path quoter, TOML parser)
- One concept per test — split rather than chain assertions
- Use `t.Helper()` in test helpers
- Use `t.TempDir()` for any filesystem fixtures (auto-cleanup after test)
- Never call `time.Now()` in tested code — pass a `time.Time` parameter or use a `clock` interface so tests can inject fixed timestamps
- Coverage target per `architecture.md` `[Testing — Unit]`: at least 60% via `go test ./... -cover`

## Integration test patterns (pytest + pywinauto)
- **Lifecycle (per test):** `agent-run.ps1 build` → `start --agent-mode` → poll `GET http://localhost:27773/status` until `ready=true` (30s timeout) → exercise UI → assert on logs/clipboard/files → `agent-run.ps1 kill`
- **UI driving:** `pywinauto.keyboard.send_keys("^+s")` for hotkeys; `pywinauto.mouse.press/move/release` with absolute virtual-screen coordinates for overlay drag
- **Clipboard reads:** `win32clipboard.OpenClipboard()` + `GetClipboardData(CF_UNICODETEXT)` — never assert on byte equality of image data
- **Log assertions:** parse `logs/agent-latest.jsonl` line-by-line; assert on `event` enum values from `internal/log/events.go`
- **Status endpoint contract:** retry on `200 + ready=false` and on connection refused; fail immediately on `503` or `404`; fail on 30s timeout
- **PID file format:** `.agent-running` is a single decimal integer on one line — no JSON, no whitespace

## Determinism
- Seed any random operations with a fixed seed
- Use fixed timestamps (`2026-01-01T00:00:00.000Z`) in unit tests, never wall-clock time
- Per-test isolation: each integration test starts a fresh agent process via `agent-run.ps1 start`; never share state between tests
- Single-instance mutex requires teardown discipline — every test must `kill` before the next `start` or the named mutex blocks

## Test data
- Use factory functions, not raw struct literals, for `config.Config` test fixtures so adding a field doesn't break unrelated tests
- No real user paths in tests — use `t.TempDir()` (Go) or `tmp_path` fixture (pytest)
- No PII in any fixture, ever

## Running tests
- **All Go unit tests:** `go test ./... -cover`
- **Single Go package:** `go test ./internal/capture -run TestFormatFilename_WithMillisecondCollision -v`
- **Race detector:** `go test ./... -race` (run before merging anything that touches goroutines or shared state)
- **Integration suite:** `pytest tests/integration/ -v` (requires Windows runner with UIA enabled, agent built in non-CGO mode)
- **Single integration test:** `pytest tests/integration/test_clipboard.py::test_clipboard_swap_with_spaces -v`
- **CI:** `go test` runs on every push; `pytest` runs only when unit tests pass and CGO_ENABLED=0 is verified

## UIA prerequisite
- **GitHub Actions `windows-latest` is assumed UIA-enabled** as of 2026-01. If UIA is unavailable, pytest fails (no soft-skip, no fallback to legacy `win32` backend) — see `architecture.md` `[Verification Profile: desktop-windows]`. Check runner image release notes if pytest jobs start failing in fixture init

## Coverage expectations
- Aim for meaningful coverage of the four pure-function packages: `capture/filename.go`, `clipboard/path_quoter.go`, `config/config.go`, `status/handler.go` (JSON marshaling)
- Win32 wrappers (`tray`, `overlay`, `hotkey`, real `clipboard` Open/Close, `toast`) are covered via integration tests, not unit tests — UIA is the only reliable way to test those
- Don't write tests just to hit coverage numbers; integration tests catch what unit tests structurally cannot

## Session Additions
_This section is owned by `/wrap-session`. setup-project preserves content added here on re-run._

- 2026-04-17: `t.Cleanup` ordering gotcha on Windows — when a test opens a long-lived file handle (e.g., via `logger.Initialize`, `os.OpenFile`) AND uses `t.TempDir()`, register the close cleanup AFTER the `t.TempDir()` call. Cleanups run LIFO, so the later-registered Close runs BEFORE TempDir's RemoveAll; the opposite order leaves the handle open and RemoveAll fails with `The process cannot access the file because it is being used by another process`. Example: `tmp := t.TempDir(); t.Cleanup(func() { _ = logger.Close() })`. Applies to any `*_test.go` that opens OS resources under a `t.TempDir()`.
