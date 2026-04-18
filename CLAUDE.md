# clip-clap

<!-- GENERATED:setup start -->

## Overview
<!-- GENERATED:setup:overview start -->
Minimalist Windows tray utility: hotkey-triggered area-select screenshot, saves PNG to a configured folder, swaps the clipboard to the auto-quoted absolute path so it can be pasted directly into a console (Windows Terminal, VS Code, WSL, SSH).

**Stack:** Go 1.23+ (CGO_ENABLED=0, pure Win32 via `golang.org/x/sys/windows`), TOML config, `log/slog` JSON logs, single static `.exe` for Windows 10/11.

**Key directories:**
- `cmd/clip-clap/` — `main.go` entry point, wires subsystems
- `internal/` — subsystem packages (tray, overlay, hotkey, capture, clipboard, toast, config, logger, status, lasterror)
- `tests/integration/` — pytest + pywinauto Windows UI integration tests
- `scripts/` — `agent-run.ps1` test harness (build/start/status/logs/kill)
- `assets/` — `app.ico`, `app.manifest` (Windows 10/11 compat, per-monitor DPI)
<!-- GENERATED:setup:overview end -->

## Purpose
<!-- GENERATED:setup:purpose start -->
A personal-scale productivity tool for developers who frequently share screenshots with terminal-based AI assistants or paste them into chats and docs. The active hotkey-triggered model puts the user in control — no passive clipboard-watching, no false positives. Replaces the multi-step "screenshot → save → find → drag" workflow with a single hotkey + drag.
<!-- GENERATED:setup:purpose end -->

## Workflow
<!-- GENERATED:setup:workflow start -->
**Key commands:**
- `go build -ldflags="-H windowsgui -s -w" -o clip-clap.exe ./cmd/clip-clap` — production build (CGO_ENABLED=0)
- `go test ./... -cover` — unit tests with coverage
- `pwsh ./scripts/agent-run.ps1 build|start|status|logs|kill` — verification harness
- `pytest tests/integration/` — pywinauto integration tests (requires running agent + UIA)
- `gofmt -w .` — format all Go files

See `.claude/docs/workflow.md` for full development workflow.
See `.claude/docs/commands.md` for complete command reference.
<!-- GENERATED:setup:workflow end -->

## Architecture
<!-- GENERATED:setup:architecture start -->
Single-binary, single-process, single-user. All UI surfaces (systray, overlay, hotkey) are direct Win32 via `golang.org/x/sys/windows` — no `lxn/walk` (frozen), no CGO. A shared Win32 message pump dispatches `WM_HOTKEY`, overlay drag events, and tray menu interactions on one UI thread. Subsystems are isolated under `internal/{tray,overlay,hotkey,capture,clipboard,toast,config,logger,status,lasterror}`; each runs on its own goroutine where applicable, with errors funneled through `slog` to `logs/agent-latest.jsonl`. Win32 UI constants not exported by `x/sys/windows` v0.24.0 (`MOD_*`, `VK_*`, `WM_*`, `RegisterHotKey`, `Shell_NotifyIcon`, etc.) are defined locally per subsystem and procs are lazy-loaded via `windows.NewLazySystemDLL(...).NewProc(...)` — see architecture.md §[Win32 API Surface].

**Primary source:** architecture.md (imported below).
<!-- GENERATED:setup:architecture end -->

<!-- GENERATED:setup:imports start -->
@.andromeda/architecture.md
@.andromeda/design-system.md
@.andromeda/security-plan.md
<!-- GENERATED:setup:imports end -->

## Critical Warnings
<!-- GENERATED:setup:warnings start -->
- **`CGO_ENABLED=0` is mandatory** in every build. CI fails loudly if it leaks to `1`. CGO breaks cross-compilation, requires MinGW, triggers Defender false-positives on unsigned binaries.
- **TOML config strict mode** rejects unknown keys — typos in `config.toml` cause `config.error` events at startup. Add new keys with care; deprecated keys must be removed cleanly.
- **Single instance enforced** via Windows named mutex `Local\ClipClapSingleInstance` (per-session namespace; changed from `Global\` per security-plan) — a second instance exits with `single_instance.violation`. Tests must clean up the prior process before launching a new one.
- **Clipboard reentry guard (500ms)** silently drops clipboard-change events from the app's own write — legitimate external clipboard activity in that window is also dropped. Documented trade-off; do not extend the window.
- **Status endpoint is loopback-only and `--agent-mode`-gated** (default off). It is a test hook, not a product API — never expose externally, never add auth, never proxy.
<!-- GENERATED:setup:warnings end -->

## Deeper Topics
<!-- GENERATED:setup:deeper-topics start -->
On-demand references in `.claude/docs/`:
- `stack.md` — tech stack with versions and sources
- `conventions.md` — naming, data model, error handling patterns
- `commands.md` — complete command reference
- `gotchas.md` — known issues and workarounds
- `workflow.md` — development and git workflow
- `session-learnings.md` — learnings curated by `/wrap-session`

Path-scoped rules in `.claude/rules/`:
- `security.md` — universal secrets / credentials / input rules
- `testing.md` — Go `testing` + pytest/pywinauto conventions
- `observability.md` — `slog` event schema, structured logging discipline

For complete Andromeda documentation: `/andromeda-help`
<!-- GENERATED:setup:deeper-topics end -->

<!-- GENERATED:setup end -->

<!-- USER:session-learnings start -->
## Session Learnings
_This section is curated by `/wrap-session`. Learnings accumulated across work sessions will appear here. Do not edit directly during wrap-session runs — changes will be preserved but wrap-session appends new entries below._

- 2026-04-17: When the user has custom `/<skill>` commands available (`/implement`, `/andromeda-*`, `/wrap-session`, etc.) for a workflow, USE them instead of rolling your own execution path — even if the plan is clear and directly executable. User values pipeline consistency and the skills encode contract details (branch strategy, progress markers, Final-N verification recipes by project type, commit-prefix rules) that hand-rolled execution silently skips. Check available skills before proceeding. (confidence 0.9)
- 2026-04-18: When a large implementation task is ahead and you offer the user a scope-tradeoff choice (MVP vs full, "options A/B/C" with varying fidelity, "ship core now / polish later"), user consistently chooses the highest-fidelity option. Phase 3 gamma plan was offered as Option A (functional-minimum), Option B (split across sessions), or Option C (full plan including fade animations, GDI text micro-bar, tick-marks, signature flash) — user picked C. Implication: don't pre-optimize for MVP or propose simplifications as the default. Offer full implementation as primary path, list simplifications as alternatives only if they genuinely help (e.g., the item is infrastructure-blocked or the full path truly can't fit in one session). Respect the user's taste for craftsmanship. (confidence 0.8)
<!-- USER:session-learnings end -->
