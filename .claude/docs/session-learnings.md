# Session Learnings

_This file is curated by `/wrap-session`. Learnings captured here are too detailed or specific for CLAUDE.md but worth preserving as reference material for future sessions._

_Entries are added in reverse chronological order (newest first). Each entry has an ISO date, short title, and body._

_This file is entirely wrap-session's territory. `/setup-project` creates it if missing but NEVER regenerates it. Manual edits are preserved across all Andromeda skill runs._

---

## 2026-04-18 — Defer plan steps that duplicate `/implement` finalization or that publish to remote state

Gamma plans sometimes include git operations as explicit steps (e.g., Phase 5 had Step 16 "Commit all Phase 5 changes to main", Step 17 "Dry-run AC #9 on a pushed feature branch + `gh workflow run`", Step 18 "`git tag v1.0.0 && git push --tags` triggers real release"). `/implement`'s Final 6–8 already handles `git add -A` + commit + feature-branch push + `gh pr create` — so Step 16 **overlaps** with the skill's built-in flow. Steps 17 and 18 affect remote/public state (pushed branches, workflow runs, a real v1.0.0 GitHub Release + Scoop manifest bump) — these are **user-triggered** post-merge actions, not implementer ones.

**Working pattern during `/implement`:**

1. Read the plan fully. Flag steps whose action category is `git commit`, `git push`, `git tag`, `gh release create`, `gh workflow run`, `scoop push`, or similar remote-affecting verbs.
2. For commits/pushes: let `/implement`'s Final 6–8 handle them via feature-branch + PR. Do NOT execute them as plan steps.
3. For tag pushes, workflow dispatches against public branches, or anything that visibly affects collaborators: **announce in the report that these are deferred to user action after PR merge**, and do NOT execute. The user can invoke them intentionally once they've reviewed the merged change.
4. Execute the code/doc/config steps normally (file creates/modifies + Verify). Phase Verification (Final 5) exercises the ldflag + YAML structure locally without requiring the remote workflow to run.

**Why:** tag pushes to `v*.*.*` trigger the actual release workflow on the first successful push — there is no "preview" or rollback. Push amplification via workflow_dispatch against a real branch publishes a workflow run attached to the repo history permanently. These side effects are too large for an orchestrator to take without user-explicit confirmation, even if the plan prescribes them.

**Applies to:** every future `/implement` run on clip-clap where the gamma plan includes final-phase git/release operations. Also applies cross-project to any Andromeda gamma plan that prescribes commits, tags, pushes, or `gh` commands as steps.

See: this session's Phase 5 `/implement` — Step 16 satisfied by PR #6 via Final 6–8, Steps 17–18 deferred to post-merge user action.

---

## 2026-04-18 — Python-based patch application is more reliable than the Edit tool for multi-line sub-agent patches with fenced code

The Edit tool fails with "File has been modified since read, either by the user or by a linter" when multiple Edit calls race against linter/hook revisions — observed repeatedly in this session's gamma Iteration Loop (Phase 4) and Security Pass (Phase 2c). Additionally, sub-agent patches often contain multi-line fenced code blocks (inner ```bash, ```json, ```yaml), and the Edit tool's `old_string` must match the plan-draft.md byte-for-byte including nested fences — off-by-one whitespace or line-ending differences cause zero-match failures.

**Working pattern for orchestrators applying sub-agent patches:**

1. Save the sub-agent's raw output to `{run_dir}/.raw-<phase>-patches.md`.
2. Parse patches with a regex that understands the sub-agent's `### Patch N` + `**Old:**` + fenced block + `**New:**` + fenced block structure. Outer-fence-aware parsing handles inner ` ```bash ` / ` ```json ` correctly.
3. Write a one-off Python script at `{run_dir}/apply-<phase>.py` that does:
   ```python
   text = plan_file.read_text(encoding='utf-8')
   for label, old, new in patches:
       count = text.count(old)
       if count == 1:
           text = text.replace(old, new, 1)
       else:
           print(f"FAILED: {label} ({count} occurrences)")
   plan_file.write_text(text, encoding='utf-8')
   ```
4. Run with `PYTHONIOENCODING=utf-8 python apply-<phase>.py` (Windows cp1252 default corrupts `→` / `—` / `✓` in sub-agent content — UTF-8 is mandatory).
5. Delete the helper script after successful application; the raw patches file stays for audit.

**Orphan-fence cleanup pass:** sub-agents sometimes leave an extra ` ``` ` line at the end of New content (captured inside the outer Old/New fence delimiters). A post-apply scan for fence-count parity and a second pass that removes lone ` ``` ` lines preceded by a `- bullet` and followed by blank + `### Step` header fixes this. Ran ~5 times this session successfully.

**Anti-pattern:** do NOT use the Edit tool for iteration patches where the Old block is > 10 lines, contains fenced content, or has been modified by a prior Edit/hook in the same session. Favor direct Write + Python substitution.

**Applies to:** every Andromeda orchestrator skill that spawns sub-agents returning patches (`/andromeda-gamma` Phases 2a/2b/2c/2d/4/5, `/andromeda-omega` iteration loop, `/andromeda-epsilon` iteration loop). The two-line `text.read_text() / patches.append / text.replace / text.write_text` loop is the standard application mechanism.

See: this session's `apply-phase5.py` + `apply-iter1.py` / `apply-iter2.py` (all deleted after use; raw patches kept at `.raw-qa-patches.md`, `.raw-security-patches.md`, iteration snapshots at `iteration-N.md`).

---

## 2026-04-18 — PowerShell `function kill`/`function start` silently collide with built-in aliases → cryptic "missing mandatory parameters" errors

PowerShell ships two built-in aliases that shadow commonly-named user functions:

- `kill` → `Stop-Process` (requires `-Id` or `-InputObject` or `-Name`)
- `start` → `Start-Process` (requires `-FilePath`)

When a script defines `function kill { ... }` AND dispatches it by bare name inside a `switch` / `if` block (e.g., `'kill' { kill }`), PowerShell's alias-vs-function resolution can unexpectedly dispatch to the alias, not the user function. The symptom is a cryptic `agent-run.ps1: Cannot process command because of one or more missing mandatory parameters: Id.` / `...FilePath.` error from the alias target — with NO stack trace pointing to the real function. The user's `Write-Host` at the top of `function kill` never executes, so there's no easy diagnostic breadcrumb.

**Working fix:** rename to non-colliding names: `killAgent`, `startAgent`. Update the switch dispatcher accordingly. The fix is trivial once identified; the diagnostic pain is entirely in detection.

**Why is this not always observed?** PowerShell's resolution order depends on scope, how the script was sourced, and whether the function was defined before or after the alias. In clip-clap's Phase 3 harness, the scripts APPEARED to work during initial smoke-testing because the dispatcher was invoked via `-Command { kill }` from an interactive session where aliases were resolved differently. Phase 4 re-exercising the harness via `pwsh scripts/agent-run.ps1 kill` as a standalone script invocation is what surfaced the collision.

**Applies to:** any `.ps1` script in clip-clap's `scripts/` directory. Future scripts (e.g., Phase 5 release pipeline, any CI helper) MUST avoid: `kill`, `start`, `stop`, `select`, `sort`, `group`, `where`, `foreach`, `echo`, `cd`, `gc`, `iwr`, `sc`, `type`, `rm`, `cp`, `mv`, `ls`, `cat`, `ps`, `gp`, `sp`. Prefer project-specific verb+noun names like `killAgent`, `startAgent`, `buildBinary`.

Grep to detect the anti-pattern before commit:
```bash
grep -nE '^function (kill|start|stop|select|sort|group|where|foreach|echo|cd|gc|iwr|sc|type|rm|cp|mv|ls|cat|ps|gp|sp)\b' scripts/*.ps1
```

See: `scripts/agent-run.ps1::killAgent` (historical: was `function kill`), `scripts/agent-run.ps1::startAgent` (historical: was `function start`).

---

## 2026-04-18 — Stray background processes from prior sessions hold Windows named mutexes → break mutex-dependent Go tests in the next session

clip-clap's single-instance guard is a named `Local\ClipClapSingleInstance` mutex created in `cmd/clip-clap/main.go::run()`. Go unit tests `TestDebugFlag_ExitsZero` + `TestDebugFlag_ResolvesToLevelDebug` exercise the full `run()` path (including mutex creation). If a stray `clip-clap.exe` from a prior session's smoke test is still running, it holds the mutex; the test's `windows.CreateMutex` returns `ERROR_ALREADY_EXISTS`, `run()` takes the "another instance is already running" exit path (code 1), and the test fails with `run with --debug should exit 0; got 1`.

The stray process is invisible to `git status`, doesn't show in any open-editor listing, and the named mutex itself is not enumerable. The symptom (test returns 1 instead of 0) gives no hint about process residue — it's easy to misattribute the failure to something just-changed in `run()` (the thing that was just edited).

**Detection:** `powershell -Command "Get-Process clip-clap -ErrorAction SilentlyContinue"` lists any live instances with PID + process name. Zero output means no residue; non-zero PID is the culprit.

**Resolution:** `powershell -Command "Stop-Process -Id <pid> -Force"` kills the stray. Tests go green immediately.

**Prevention for future sessions:**
1. `/wrap-session` should ideally emit a warning if `Get-Process clip-clap` returns any live instance at wrap time (signal that the prior smoke test didn't clean up).
2. `scripts/agent-run.ps1 kill` + its `taskkill` fallback path (added in Phase 4) is the canonical cleanup — always call at end of smoke testing.
3. A pre-test Go helper that calls `taskkill /IM clip-clap.exe /F` would be belt-and-suspenders, but adding OS-specific cleanup to `go test` is overkill for a solo-dev project. Ad-hoc manual cleanup via the PowerShell snippet above is the current pattern.

**Applies to:** any Go test in clip-clap that exercises `run()` (i.e., goes through mutex creation). Currently: `cmd/clip-clap/main_test.go::TestDebugFlag_ExitsZero`, `TestDebugFlag_ResolvesToLevelDebug`. Future Phase 5 tests exercising `run()` will have the same vulnerability.

See: `cmd/clip-clap/main.go::run` (mutex creation), `cmd/clip-clap/main_test.go::setupTestEnv` (tests that go through the mutex path).

---

## 2026-04-18 — `taskkill /PID` (no `/F`) is a better graceful-kill primitive than `$proc.CloseMainWindow()` for HWND_MESSAGE / tray-only Win32 apps

PowerShell's `Process.CloseMainWindow()` only targets windows with a visible top-level main window. clip-clap's message pump uses `HWND_MESSAGE` parent — a hidden, message-only window that is NOT a top-level window — so `$proc.CloseMainWindow()` returns `false` and the process receives no WM_CLOSE. Result: the graceful-shutdown path (status.Shutdown → PostQuitMessage) never fires, and the `WaitForExit(5000)` times out, forcing the taskkill fallback for every kill (always 5+ seconds per kill).

**Working alternative:** `taskkill /PID $procId` (WITHOUT `/F`) sends WM_CLOSE to **all** top-level windows of the process, including message-only windows registered via `CreateWindowExW(HWND_MESSAGE, ...)`. clip-clap's wndProc receives the WM_CLOSE and runs the full graceful-shutdown flow. Observed latency: ~500ms end-to-end (vs. 5000ms for the CloseMainWindow-timeout path).

**Two-stage kill pattern (Phase 4 `scripts/agent-run.ps1::killAgent`):**
1. `taskkill /PID $procId` — sends WM_CLOSE, app handles gracefully
2. `$proc.WaitForExit(3000)` — 3-second budget for graceful exit
3. On timeout: emit `falling back to taskkill /PID <pid> /F` to stderr, invoke `taskkill /PID $procId /F`, wait 500ms

Total worst-case budget: ~3.5 seconds (well under the 5-second AC contract for `agent-run.ps1 kill`). Best-case: ~500ms.

**Why not `$proc.Kill()`?** That's equivalent to `taskkill /F` — force-terminate without giving the app a chance to run deferred cleanup (status.Shutdown drain, PID-file delete, etc.). Graceful-first is correct for the test harness because the app's shutdown codepaths are under test.

**Applies to:** any Win32 app in clip-clap that uses `HWND_MESSAGE` (the current message pump, any future background worker that registers a message-only window). Avoid `CloseMainWindow()` in PowerShell cleanup scripts; prefer `taskkill /PID` → wait → `taskkill /PID /F` fallback.

See: `scripts/agent-run.ps1::killAgent` (the two-stage kill implementation), `cmd/clip-clap/main.go::runMessagePump` (HWND_MESSAGE creation), `cmd/clip-clap/main.go::wndProc` WM_CLOSE handler (graceful-exit flow).

---

## 2026-04-18 — Use `RtlMoveMemory` for Win32 HGLOBAL / pointer-to-kernel-memory copies to avoid go vet `unsafeptr` warnings

`go vet` flags `(*T)(unsafe.Pointer(uintptr))` conversions as "possible misuse of unsafe.Pointer" because Go's GC may move managed pointers mid-expression (invalidating the uintptr). For Win32 code this is a false positive — `HGLOBAL` memory from `GlobalAlloc` + `GlobalLock` lives in the Win32 kernel heap, NEVER moves, and the uintptr-returning lazy-proc `Call()` pattern is the canonical way to receive it. But vet cannot tell the difference, and `go vet ./...` returns exit 1.

**Working pattern:** route the buffer copy through `RtlMoveMemory` (kernel32's memcpy primitive). The vet-approved pattern is `uintptr(unsafe.Pointer(&goSlice[0]))` — taking the address of a Go-managed slice IS safe and vet accepts it. By passing the HGLOBAL uintptr as-is (no type cast) and the Go-slice address via `uintptr(unsafe.Pointer(...))`, both directions of the copy stay within vet's allowed idioms:

```go
procRtlMoveMemory := kernel32.NewProc("RtlMoveMemory")

// HGLOBAL → Go slice (read)
snap := make([]uint16, n)
procRtlMoveMemory.Call(
    uintptr(unsafe.Pointer(&snap[0])),  // vet-approved: addr of Go slice
    locked,                              // uintptr from GlobalLock: no cast
    uintptr(n*2),
)

// Go slice → HGLOBAL (write)
withTerm := append(payload, 0)
procRtlMoveMemory.Call(
    dst,                                 // uintptr from GlobalLock: no cast
    uintptr(unsafe.Pointer(&withTerm[0])),
    uintptr(len(withTerm)*2),
)
```

The anti-pattern that fails vet: `unsafe.Slice((*uint16)(unsafe.Pointer(dst)), n)` — this is the direct-cast approach that would be ergonomic in non-vet-guarded contexts but triggers the warning. Phase 3 `internal/clipboard/clipboard.go` initially used this and `go vet` exited 1; refactored to RtlMoveMemory and vet went clean.

**Applies to:** any future Win32-heap pointer manipulation in clip-clap (future `internal/overlay` DIB pixel reads, `internal/clipboard` image-format clipboard writes, etc.). Also applies to memory obtained from `GlobalLock`, `VirtualAlloc`, `HeapAlloc`, `LocalAlloc` — basically anything that returns a uintptr representing non-Go-managed OS memory.

---

## 2026-04-18 — Function-pointer injection breaks subsystem import cycles without a shared-types refactor

Phase 3 `internal/tray` needed to query `clipboard.HasSnapshot()` (to enable/disable the "Undo last capture" menu item) AND invoke `clipboard.Undo()` (on menu click). But `internal/clipboard` already imports `internal/tray.SanitizeForTray` (to sanitize `*os.PathError` before setting lasterror). Direct import in either direction creates a cycle.

**Working pattern:** tray package exposes package-level function-pointer vars that main.go wires at startup via a `tray.SetHandlers(capture, undo func(), hasSnapshot func() bool)` entry point. No import of clipboard in tray; no import cycle. main.go (which already imports both) wires the closures:

```go
// internal/tray/tray.go
var (
    captureHandler  func()
    undoHandler     func()
    hasSnapshotFunc func() bool
)

func SetHandlers(capture, undo func(), hasSnapshot func() bool) {
    captureHandler = capture
    undoHandler = undo
    hasSnapshotFunc = hasSnapshot
}

// cmd/clip-clap/main.go startup:
tray.SetHandlers(
    func() { runCaptureFlow(hwnd) },
    func() { runUndoFlow(hwnd) },
    clipboard.HasSnapshot,
)
```

**Alternatives considered (and rejected):**
- Move `SanitizeForTray` out of `internal/tray` into a shared `internal/errs` package — viable but adds a package for one function. Function-pointer pattern preserves the single-purpose `internal/tray` boundary.
- Make clipboard self-sanitize (skip the tray helper) — ugly duplication if other subsystems grow to use the same helper.
- Replace function pointers with an interface `type TrayHandlers interface { Capture(); Undo(); HasSnapshot() bool }` — more Go-idiomatic but heavier for 3 methods.

**Applies to:** any future clip-clap subsystem that needs cross-subsystem orchestration where imports would form a cycle (e.g., `internal/status` endpoint needing to trigger `internal/capture` for a synthetic capture in agent-mode — same pattern: status exposes a handler pointer, main.go wires it).

---

## 2026-04-18 — Architecture.md reconciliation requires a second pass: Established Decisions drift out of sync with Stack table

The first-pass architecture reconciliation on 2026-04-17 (commit `c9f0a5a`, PR #3) updated the Stack table and `[Go Module Version Pinning]` list but left `## Established Decisions` and `## Cross-cutting Patterns` with stale text that contradicted the freshly-updated tables. Specifically:

1. **`[Toast Notification Library]`** still said "`stuartleeks/toast` fork as maintenance fallback" while Stack table + pinning list already said "fork NOT used per security-plan — unmaintained since 2019". A Phase-3 Gamma sub-agent reading the document end-to-end would have seen the contradiction and could have proposed re-adding the fork.

2. **`[Error Handling]`** Established Decision and **`[Error Handling — Subsystem Failures]`** Cross-cutting Pattern both referenced `event=subsystem.error` as a generic placeholder. There is no such constant in `internal/logger/events.go` — the actual enumerated events are per-subsystem (`hotkey.error`, `toast.error`, `config.error`, and the Phase-3-planned `tray.flash.error` from design-system backlog).

**Lesson:** surgical reconciliation of a multi-section document (`architecture.md` has ~10 top-level sections, each potentially restating stack facts) needs a second pass through the *prose* sections after the *table* sections are updated. Tables are easy to diff; prose is where duplicated-fact drift hides. A grep checklist helps:
```bash
grep -nE '{stale-library-name}|{stale-event-name}|{stale-version}' .andromeda/architecture.md
```
If the same fact is referenced in Stack table AND in an Established Decision AND in a Cross-cutting Pattern, ALL three sites need updating together.

**Applies to:** every future `/andromeda-gamma` run on clip-clap, plus any scope-level reconciliation via `/andromeda-sigma`. Budget ~15 minutes for a prose-section second pass after tables are updated — it's the difference between a Phase-N+1 Gamma planning against consistent facts vs contradictory ones.

**This session's specific fixes** (applied locally to gitignored `.andromeda/architecture.md`):
- `[Toast Notification Library]`: rewrote to reflect pseudo-version pin + explicit fork rejection per security-plan
- `[Error Handling]`: replaced generic `event=subsystem.error` with enumerated-events reference + `internal/lasterror` API mention
- `[Error Handling — Subsystem Failures]`: same enumerated-events replacement

Grep state after fixes: `stuartleeks` appears 3× all in "NOT used"/"unmaintained" anti-context; `subsystem.error` appears 1× as a prescribed anti-example ("NOT a generic `subsystem.error` placeholder") — documenting the negative pattern to prevent re-introduction by a future sub-agent that might re-derive it from Phase-0 language.

---

## 2026-04-17 — `/setup-project` on mature projects: prefer surgical updates over full template regen

The `/setup-project` skill regenerates `.claude/rules/*.md` and `.claude/docs/*.md` from generic templates, preserving only `## Session Additions` at the bottom. On a Phase-0 fresh setup this works well. On a Phase-N+ project where rule/doc files have accreted hand-curated content (specific commands, Phase gotchas, framework-specific patterns), full template regen DELETES that content and replaces it with shallow template stubs like `**Unit tests:** {test framework from architecture.md}`.

For clip-clap specifically, `.claude/rules/testing.md` and `.claude/rules/observability.md` had rich project-specific content (UIA prerequisite notes, event-enum rules, clipboard-reentry specifics) above `## Session Additions` that would have been destroyed by a template regen. `.claude/docs/stack.md` similarly had a 70-line Core Frameworks & Libraries block with per-library rationale that the template equivalent reduces to a single placeholder line.

**Working workflow when `/setup-project` is re-invoked on a mature project to sync with an updated `architecture.md`:**
1. Read the skill's phases to understand intent.
2. Recognize which tracked files have hand-curation above `## Session Additions` vs which are still at template-stub level.
3. Do surgical edits (fix stale refs like `internal/log` → `internal/logger`, `Go 1.22+` → `Go 1.23+`, `Global\` → `Local\`) rather than wholesale template regen.
4. Explain the divergence to the user in the final report and commit with a `chore:` prefix.

The template regen IS the right call when the user explicitly asks for a "factory reset" of Claude config (e.g., the project pivoted languages/frameworks). Default assumption for a running project: surgical.

**Applies to:** any future re-invocation of `/setup-project` on clip-clap, and the same judgement applies to future scopes created via `/andromeda-sigma`.

---

## 2026-04-17 — Architecture-reconciliation file checklist

When `.andromeda/architecture.md` is updated to reconcile post-implementation deviations (e.g., after a phase's `/implement` produced 4+ adaptations that drifted from the plan), the following git-tracked files typically need matching surgical updates. Checklist derived from the Phase-2 reconciliation that produced 8 architecture deviations → 6 tracked-file edits → 20 inserts/18 deletes:

- `CLAUDE.md` `GENERATED:setup:overview` — stack version one-liner, key-directories package list
- `CLAUDE.md` `GENERATED:setup:architecture` — subsystem package list, architectural patterns (Win32 surface notes, etc.)
- `CLAUDE.md` `GENERATED:setup:warnings` — mutex name, single-instance rules, critical-warning stack refs
- `.claude/rules/*.md` — `paths:` frontmatter (for renamed packages) AND body references to specific package paths
- `.claude/docs/stack.md` — Core Frameworks & Libraries table (versions, pseudo-versions, library fork status per security-plan)
- `.claude/docs/gotchas.md` — Go/toolchain version pin notes, migration-specific entries
- `.claude/agents/code-reviewer.md` — project-specific checklist rules referring to package paths or event enums

`.claude/docs/session-learnings.md` is wrap-session's territory and is never touched by setup-project/reconciliation. `.claude/docs/conventions.md`, `.claude/docs/commands.md`, `.claude/docs/workflow.md` rarely need touching for version/path deviations because they're framed at a level above specific versions.

**Grep one-liner to catch remaining stale refs after a reconciliation pass:**
```bash
grep -rnE '{stale-version}|{stale-package-path}|{stale-mutex-name}' .claude/ CLAUDE.md
```

Historical "bumped from X" / "renamed from Y" mentions are expected noise in the output; the grep's intent is to find FACTUAL stale references (text that asserts a current-but-wrong value), not NARRATIVE ones (text that documents history).

**Applies to:** every future reconciliation pass on clip-clap — typically triggered after `/implement` completes a phase and produces ≥3 plan-to-code adaptations.

---

## 2026-04-17 — `.andromeda/` is gitignored; architecture.md edits live locally

The `.andromeda/` directory (containing `architecture.md`, `design-system.md`, `security-plan.md`, `masterplan.md`, `project.yaml`, and `runs/*`) is in `.gitignore` line 7 per the "HANDOVER 2026-04-16" convention on clip-clap. Consequences:

- Direct edits to `.andromeda/architecture.md` are locally effective without any commit — the file is `@`-imported from `CLAUDE.md` into every Claude Code session on this machine.
- Those edits do NOT propagate to other machines, fresh clones, or CI — because nothing tracks them in git.
- When you reconcile architecture deviations (e.g., after a phase's implementation drift), the authoritative fix is in `.andromeda/architecture.md` locally, but the git-tracked half (CLAUDE.md, `.claude/rules/*`, `.claude/docs/*`, `.claude/agents/*`) must be updated separately and committed via a feature branch + PR.
- clip-clap is single-developer, so the local-vs-tracked divergence is tolerable. A multi-developer port would need to un-gitignore `.andromeda/` OR formalize the "architecture is local-only source of truth, tracked config is synced projection" workflow with a CI check.

**Practical implication for wrap-session and new-session:** never assume `git status` covers architecture edits. If a session touched `.andromeda/architecture.md` and no related commit appears, that's expected — check whether the architecture delta has been propagated to tracked config (CLAUDE.md etc.) separately.

---

## 2026-04-17 — Prefer `ExtractIconW` over `LoadIconW(id)` for `goversioninfo`-embedded icons

`goversioninfo` v1.5.x packs `assets/app.ico` as an `RT_GROUP_ICON` resource whose name varies between library versions — neither `IDI_APPLICATION` (32512, a user32.dll sentinel) nor numeric ID 1 (the pattern documented in some goversioninfo sources) resolved for clip-clap's embedded icon. `LoadIconW(hInstance, MAKEINTRESOURCE(id))` fails with `"The specified resource type cannot be found in the image file"` even after regenerating `resource.syso` via `go generate ./cmd/clip-clap`.

**Working pattern:** resolve the .exe path via `GetModuleFileNameW(moduleHandle, &buf, len(buf))`, then call `ExtractIconW(hInstance, exePath, 0)` — extracts the first icon by INDEX, agnostic to the resource-name choice. Handle return values: `0` means no icon found, `1` is an error sentinel (file not an exe/dll/ico), anything else is a valid `HICON`. Lazy-load both procs from shell32.dll (`ExtractIconW`) and kernel32.dll (`GetModuleFileNameW`) — neither is exported by `golang.org/x/sys/windows` v0.24.0.

**Applies to:** any Phase 2+ clip-clap subsystem that loads the tray icon (currently `internal/tray/tray.go::RegisterIcon`) and future Phase 3 amber-flash swap when it needs to re-resolve the deep-ink icon.

See: `internal/tray/tray.go::RegisterIcon` (production code), `internal/tray/win32.go` (`procExtractIconW`, `procGetModuleFileNameW` lazy loaders).

---

## 2026-04-17 — `golang.org/x/sys/windows` v0.24.0 omits most Win32 UI constants and procs

`golang.org/x/sys/windows` v0.24.0 does NOT export the Win32 UI-surface constants or procs that clip-clap's tray/overlay/hotkey subsystems need: `MOD_ALT`/`MOD_CONTROL`/`MOD_SHIFT`/`MOD_WIN`, `VK_*` (F1–F12, SPACE, PRIOR, NEXT, HOME, END, INSERT, DELETE), `WM_CLOSE`/`WM_QUIT`/`WM_COMMAND`/`WM_HOTKEY`/`WM_RBUTTONUP`, plus `RegisterHotKey`, `Shell_NotifyIconW`, `TrackPopupMenuEx`, `CreateWindowExW`, `RegisterClassExW`, `GetMessageW`, `DispatchMessageW`, and essentially the entire user32.dll UI API.

**Working pattern:** define constants locally in each package as typed `uint32` consts with the Win32-documented values, and lazy-load procs via:
```go
var (
    user32 = windows.NewLazySystemDLL("user32.dll")
    procRegisterHotKey = user32.NewProc("RegisterHotKey")
)
```
Invoke via `proc.Call(arg1, arg2, ...)` which returns `(uintptr, uintptr, error)` — the first uintptr is the return value, the error is from `GetLastError`. This pattern is used across `internal/hotkey/hotkey.go`, `internal/tray/win32.go`, and `cmd/clip-clap/win32.go`.

**Applies to:** every future Win32-facing package in clip-clap (overlay, clipboard Win32 open/close, toast WinRT fallback). Do NOT assume a Win32 API is exported from x/sys/windows — verify with `go doc -short golang.org/x/sys/windows {name}` first; if not found, define/lazy-load locally.

See: `internal/hotkey/hotkey.go` (MOD_*/VK_* + procRegisterHotKey), `internal/tray/win32.go` (Shell_NotifyIcon + TrackPopupMenuEx surface), `cmd/clip-clap/win32.go` (window+pump surface).

---

## 2026-04-17 — `atomic.Value.Store` panics on heterogeneous concrete types; wrap in a single-type holder

`sync/atomic.Value.Store(v)` panics with `"sync/atomic: store of inconsistently typed value into Value"` if called with different concrete types across writes — even when every value satisfies the same interface. clip-clap's `internal/lasterror` package uses `atomic.Value` as an error sink that subsystems write to from multiple goroutines; subsystems publish errors of different concrete types (`*errors.errorString` from the hotkey parser, `*os.PathError` from `exec.Command`, `*fmt.wrapError` from `fmt.Errorf("...: %w", ...)`, custom struct pointers from future subsystems, etc.), so a naive `atomic.Value.Store(err)` at the call site will panic the first time subsystem B writes an `*os.PathError` after subsystem A wrote an `*errors.errorString`.

**Working pattern:** define an unexported wrapper struct with a single `error` field, and always Store that concrete type:
```go
type errorHolder struct { err error }

var LastError atomic.Value

func Set(err error) { LastError.Store(errorHolder{err: err}) }
func Get() error {
    v := LastError.Load()
    if v == nil { return nil }
    return v.(errorHolder).err
}
```
All Stores now pass the same concrete type (`errorHolder`); the `err` field inside can hold any error type. `TestSet_DifferentConcreteTypes` in `internal/lasterror/lasterror_test.go` is the regression guard.

**Applies to:** any `atomic.Value`-based shared-state sink where multiple code paths produce values of different concrete types satisfying a common interface — not limited to errors (could be `io.Reader`, `context.Context` variants, etc.).

See: `internal/lasterror/lasterror.go::errorHolder`, `internal/lasterror/lasterror_test.go::TestSet_DifferentConcreteTypes`.

---

## 2026-04-17 — Explore sub-agents reliably prepend conversational preamble; strip-and-save-raw is the pragmatic response

When spawning Explore-type sub-agents via the Agent tool with strict `NO_PREAMBLE` validation (e.g., "first character of your response must be `#`", "no 'Based on my read...' or 'Perfect! I now have...'"), sub-agents still produce 2–3 lines of conversational preamble before the required template start. Observed 6+ times across a single `/andromeda-gamma` run (Phase 0 task extraction, Phase 1 research, Phase 2 draft, Phase 3 meta-prompt, Phase 3a refinement — every sub-agent invocation).

The pragmatic orchestrator response — authorized explicitly by the user on the first occurrence and carried forward through the run:

1. **Save the full raw sub-agent output** to `{run_dir}/.raw-<filename>.md` for audit. This preserves Pass 1–5 analysis, preamble, and any rationale that might be useful later but shouldn't land in the final artifact.
2. **Manually strip the preamble** when writing the cleaned content to the final artifact (e.g., `task.md`, `research.md`, `plan-draft.md`). The post-preamble template is usually well-filled; the sub-agent's CONTENT quality is good, only its output DRESSING fails validation.
3. **Report the residual in the orchestrator's dashboard** ("preamble stripped; raw saved to .raw-*.md") so the user sees what happened without having to read the raw file.
4. **Do NOT halt the pipeline** on NO_PREAMBLE failure alone — the halt-after-2-failures contract is appropriate for content-quality failures (missing sections, empty AC lists) but overly strict for cosmetic preamble.

This pattern applies to every Andromeda orchestrator skill that spawns sub-agents (alpha, beta, gamma, omega, epsilon, sigma). The validation framework already supports it — validation checks other than NO_PREAMBLE usually pass, and the preamble is trivially strippable.

See: `.andromeda/runs/2026-04-17T14-47-51-gamma-phase-1/.raw-*.md` (seven concrete examples of the pattern and its resolution in a single run).

---

## 2026-04-17 — Gamma plans can contain internal contradictions between test specs and AC contracts

`/andromeda-gamma` extracts test specifications (Phase 2a QA sub-agent) and step specifications (Phase 2 first-draft sub-agent) independently from different source sections — `task-tests.md` for tests, `task.md` for ACs. The two extractions are not cross-validated, so the resulting `plan-draft.md` can contain a test whose premise contradicts the AC contract the test is nominally meant to cover.

**Concrete example from clip-clap Phase 1 (2026-04-17):** task-tests.md specified `TestLoad_AutoCreateOnMissingFile` with setup "set temp config path via `CLIP_CLAP_CONFIG` env var to a non-existent file... assert the file was created with default values". But task.md + AC #7 require CLIP_CLAP_CONFIG + missing-file to RETURN AN ERROR (only the DEFAULT `%APPDATA%\clip-clap\config.toml` path auto-creates on first run). The test's setup directly contradicted the AC contract.

**Resolution during /implement:** trust the AC contract (architecture-level authority); adjust the test's SETUP to reach the same INTENT through the correct path. Here: redirect `AppData` and `XDG_CONFIG_HOME` env vars to a TempDir so `os.UserConfigDir()` returns the temp location, then call `Load()` without setting `CLIP_CLAP_CONFIG` — exercises the default-path auto-create that AC #1 intends. The CLIP_CLAP_CONFIG-missing case is tested separately by `TestLoad_CustomConfigMissingFile` per AC #7 (now two distinct tests for two distinct contracts).

**Lesson for future `/implement` runs:** when a test spec in a plan seems to conflict with an AC it's meant to cover, **trust the AC** (architecture-level) and adjust the test setup to reach the same INTENT through the correct path. Document the adjustment in the implement-progress.md audit trail (§Adaptations / deltas from plan). If the contradiction is systemic (multiple tests misaligned), halt and ask the user before proceeding.

See: `.andromeda/runs/2026-04-17T14-47-51-gamma-phase-1/implement-progress.md` §Adaptations (finding #1).

---

## 2026-04-17 — Go pseudo-versions from Andromeda plans may not exist on proxy.golang.org

When an Andromeda gamma plan specifies a Go module at a pseudo-version like
`v0.0.0-YYYYMMDDHHMMSS-<12charSHA>`, the SHA suffix may be synthetic or stale
and not exist on the Go proxy. Architecture-table entries with version strings
that were never tagged upstream (e.g., `v0.0.4` for a module that only has
`master` commits) fall in the same trap.

**Pragmatic workflow before writing `go.mod` from a plan:**

1. For **tagged modules**, query: `go list -m -versions <module>` and confirm
   the plan's version is in the list.
2. For **pseudo-versioned modules**, query:
   `curl -sS https://proxy.golang.org/<module>/@latest`
   to get the actual latest commit SHA, and compare against the plan's SHA.
3. Resolve discrepancies by using the real latest; record the actual version
   in `docs/dependencies.md` with a review-date audit note per security-plan
   §Dependency Security.

**Examples from clip-clap Phase 0 (2026-04-17):**
- Architecture and security-plan listed `go-toast/toast v0.0.0-...01e3ca3626d8`
  — synthetic SHA, proxy returned `unknown revision`. Real latest was
  `01e6764cf0a4` from 2019-02-11.
- Architecture listed `kbinani/screenshot v0.0.4` — no such tag; actual
  latest on `master` is
  `v0.0.0-20250624051815-089614a94018`.

**Also note:** `go mod tidy` removes a `require` if no code imports it. For
Phase 0 (empty package stubs, no imports), use a `//go:build tools` file
(`tools/tools.go`) with blank `_` imports to keep the deps pinned in
`go.mod`. The `tools` build tag excludes the file from regular builds.

See: `go.mod`, `docs/dependencies.md`, `tools/tools.go`

---

## Entry format

Each entry follows this structure:

```
## {ISO-date} — {short title}
{1-3 paragraphs describing what was learned, why it matters, and where it applies. Reference specific files or documented decisions when relevant.}

See: `.claude/docs/...` or `internal/...` (cross-references)
```

## Tier classification

This file is **Tier 3 — on-demand**. Claude reads it when explicitly needed (debugging, planning, reviewing patterns), not at session start.

Other tiers:
- **Tier 1** (always loaded) — universal safety rules in `CLAUDE.md` `USER:session-learnings` section (critical, short)
- **Tier 2** (path-triggered) — directives in `.claude/rules/*.md` `## Session Additions` sections (loaded when matching files touched)
- **Tier 3** (on-demand) — this file (detailed reference, lazy-read)

## Promotion

When this file grows beyond ~200 lines, `/wrap-session` suggests promoting some entries to topic-specific files. Promotion is a user action, not automatic.

## Demotion from CLAUDE.md

If `CLAUDE.md` `USER:session-learnings` section gets too large (≥ 180 lines total CLAUDE.md), `/wrap-session` suggests promoting old Tier 1 entries down to this file (Tier 3) to keep CLAUDE.md within size budget.
