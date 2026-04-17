# Gotchas

_Known issues, non-obvious constraints, and architectural traps. Extracted from `.andromeda/architecture.md` Established Decisions and Cross-cutting Patterns sections by `/setup-project`._

This file is regenerated when `/setup-project` runs. Learnings discovered during work sessions go to `session-learnings.md` (curated by `/wrap-session`).

## Format

Each gotcha follows this structure:

```
## {Short title}
**What breaks:** {failure mode}
**How to avoid:** {prevention}
**Fix if broken:** {recovery, if known}
**References:** {files, lines, architecture sections}
```

## Gotchas

## CGO_ENABLED leak breaks the entire stack
**What breaks:** Setting `CGO_ENABLED=1` at any point — locally, in CI, or via a stale `go env` setting — defeats the static-binary pure-Go contract: cross-compilation breaks, MinGW becomes a hidden requirement, and Windows Defender behavioral scans start flagging the unsigned binary as suspicious.
**How to avoid:** Confirm `go env CGO_ENABLED` returns `0` before building. CI verifies this and fails loudly if it leaks. Never use C-wrapping libraries.
**Fix if broken:** `go env -w CGO_ENABLED=0` and rebuild. Investigate which dependency or PR introduced the leak (usually a CGO-using library replacing a pure-Go one).
**References:** `.andromeda/architecture.md` `[CGO Policy]`, `.claude/rules/security.md` Build & dependency hygiene

## TOML config strict mode rejects unknown keys
**What breaks:** Renaming or removing a config key in code without removing it from any user's existing `config.toml` causes startup to fail with `config.error` event. Adding a typo in code that doesn't match a real key in `config.toml` likewise fails.
**How to avoid:** When renaming a config key, ship a one-time migration that rewrites the old key to the new in user files (or document the rename in release notes). When adding new keys, give them sensible defaults so absence in old configs doesn't fail.
**Fix if broken:** User must edit `%APPDATA%\clip-clap\config.toml` to remove the rejected key, or delete the file to regenerate defaults.
**References:** `.andromeda/architecture.md` `[Config Format]`, `internal/config/config.go`

## Single-instance mutex blocks tests if not killed
**What breaks:** Integration tests start a fresh agent process per test. If the prior `agent-run.ps1 kill` didn't complete, the named mutex `Global\ClipClapSingleInstance` is still held — the new process exits with `single_instance.violation` and the test fails with confusing "agent never reached ready=true" symptoms.
**How to avoid:** Every integration test pairs `agent-run.ps1 start` with `agent-run.ps1 kill` in `try`/`finally` (or pytest fixture teardown). The harness must verify the prior PID is gone before launching.
**Fix if broken:** `taskkill /F /PID $(cat .agent-running)` then `Remove-Item .agent-running` and re-run.
**References:** `.andromeda/architecture.md` `[Single-Instance Guard]`, `.claude/rules/testing.md`

## Clipboard reentry guard silently drops legitimate writes
**What breaks:** During the 500ms window after the app writes the captured path, ANY clipboard-change event is dropped — including legitimate copies the user makes from another app. The user may "copy text" and not see it appear in their clipboard if it lands inside the guard window.
**How to avoid:** Documented trade-off; do not extend the window. The guard is per-`capture_id` (not global), so two captures don't widen the effective window beyond 500ms each. Tests verify the guard releases on schedule.
**Fix if broken:** N/A by design — if the user runs into this, they re-copy after the toast appears.
**References:** `.andromeda/architecture.md` `[Clipboard Reentry Guard]`, `.claude/rules/observability.md` Privacy section

## Hotkey already bound by another app fails silently
**What breaks:** If the configured hotkey (`Ctrl+Shift+S` by default) is already bound by another application, `RegisterHotKey` returns an error. The app does NOT show a toast — only a `hotkey.error` log event and a "Last error" tray menu entry. Users may think the app is broken.
**How to avoid:** Document this in README. The tray menu's "Capture" entry remains functional even when the hotkey is unavailable, so the user has a fallback.
**Fix if broken:** User edits `config.toml` `hotkey` field to a different combination and restarts the app.
**References:** `.andromeda/architecture.md` `[Hotkey Message Pump Contract]`, `internal/hotkey/hotkey.go`

## DPI awareness must be in `app.manifest` or overlay misrenders on HiDPI
**What breaks:** Without per-monitor DPI awareness declared in `assets/app.manifest`, the transparent overlay's `UpdateLayeredWindow` bitmap is scaled by Windows on 2K/4K displays, causing visual artifacts and incorrect drag-rectangle hit-testing.
**How to avoid:** Keep the per-monitor DPI awareness GUID in `app.manifest`. The `goversioninfo` codegen step embeds it into `resource.syso`. Don't strip the manifest "to simplify the build."
**Fix if broken:** Restore the manifest, regenerate `resource.syso` via `go generate ./...`, rebuild.
**References:** `.andromeda/architecture.md` `[Resource Embedding]`, `assets/app.manifest`

## UIA must be available on the test runner
**What breaks:** `pywinauto` v0.6.8 with the UIA backend requires Windows UI Automation services to be enabled. If UIA is unavailable on the runner (rare on `windows-latest` as of 2026-01, but possible after image updates), pytest fails during fixture initialization — no soft-skip, no fallback.
**How to avoid:** Check GitHub Actions runner image release notes when pytest jobs start failing in fixture init. If the image image disables UIA, pin to a prior runner image version.
**Fix if broken:** Pin runner image, or open issue on `actions/runner-images` repo. No code workaround in this project.
**References:** `.andromeda/architecture.md` `[Verification Profile: desktop-windows]`, `.claude/rules/testing.md` UIA prerequisite

## go.sum drift breaks reproducible CI builds
**What breaks:** `go mod tidy` is not idempotent across Go versions in subtle cases (e.g., transitive dependency selection). If `go.sum` differs between developer machines or between local and CI, CI fails on `git diff --exit-code go.sum`.
**How to avoid:** Always commit the `go.sum` produced by the CI's pinned Go version (1.22). When bumping Go versions, update CI first, then re-tidy locally.
**Fix if broken:** Run `go mod tidy` with the same Go version as CI, commit the result.
**References:** `.andromeda/architecture.md` `[Go Module Version Pinning]`, `go.mod`, `go.sum`

## Save folder must exist or capture fails
**What breaks:** If `save_folder` in config points to a directory that doesn't exist, the first capture fails with `capture.failed` event and no PNG is written. Users may not realize the path is misconfigured.
**How to avoid:** `internal/capture/capture.go` proactively calls `os.MkdirAll(savePath, 0o755)` before each capture (idempotent). If the folder exists but is read-only, capture still fails — the tray "Last error" surfaces this.
**Fix if broken:** User fixes `save_folder` in `config.toml` to a writable directory and restarts (or just retries the capture if the folder was just created).
**References:** `.andromeda/architecture.md` `[First-Run Behavior]`, `internal/capture/capture.go`

## Related

- For runtime-discovered learnings, see `.claude/docs/session-learnings.md` (curated by `/wrap-session`)
- For path-specific rules, see `.claude/rules/*.md`
- For architectural decisions and rationale, see `.andromeda/architecture.md` Established Decisions section
