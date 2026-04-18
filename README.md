# Clip Clap

Minimalist Windows tray utility: hotkey-triggered area-select **screenshot**,
saves PNG to a configured folder, swaps the clipboard to the auto-quoted
absolute path so it can be pasted directly into a console (Windows Terminal,
VS Code, WSL, SSH).

**Status:** Phase 0 (foundation skeleton). Not yet feature-complete; v1.0
ships when Phases 1–5 land.

## Installation

### A. From GitHub Releases

```powershell
gh release download v1.0.0 --repo Turbolet85/clip-clap --pattern clip-clap.exe
```

Or download `clip-clap.exe` directly from the
[GitHub Releases page](https://github.com/Turbolet85/clip-clap/releases).

### B. Via Scoop

```powershell
scoop bucket add Turbolet85 https://github.com/Turbolet85/scoop-bucket
scoop install clip-clap
```

### C. First Run — Expected SmartScreen Warning

> **⚠ SmartScreen warning on first run is expected.** v1.0 ships **unsigned**,
> so Windows Defender SmartScreen will display a "Windows protected your PC"
> dialog when you first launch `clip-clap.exe`. Click **"More info"** →
> **"Run anyway"** to start the tray agent. This warning is expected for
> unsigned binaries and disappears once your local install builds reputation
> (or when v1.1+ ships code-signed via Azure Artifact Signing — Public Trust).
>
> For the latest version and release notes, visit the
> [GitHub Releases page](https://github.com/Turbolet85/clip-clap/releases).

## Usage

Press **Ctrl+Shift+S** (default hotkey) → drag a rectangle on screen →
the screenshot is saved to `%USERPROFILE%\Pictures\clip-clap\` and the
clipboard now contains the absolute path. Paste into your terminal:

```
> "C:\Users\you\Pictures\clip-clap\2026-04-17_14-30-22_481.png"
```

The tray icon flashes safelight-amber for 350 ms on each capture (your
visual confirmation). Right-click the tray icon for: Open folder, Undo
last capture, Quit.

## Configuration

Optional `%APPDATA%\clip-clap\config.toml`:

```toml
save_folder = "D:\\screenshots"
hotkey = "Ctrl+Shift+S"
auto_quote_paths = true
log_level = "INFO"
```

Environment variables override config:

- `CLIP_CLAP_CONFIG` — alternative config.toml path
- `CLIP_CLAP_SAVE_DIR` — override save folder
- `CLIP_CLAP_LOG_PATH` — redirect log output
- `CLIP_CLAP_DEBUG=1` — bump log level to DEBUG

## Development

This project is built with the Andromeda planning pipeline.
See `.andromeda/` (gitignored, local-only) for the architecture, masterplan,
design system, and security plan. See `docs/dependencies.md` for the
dependency audit trail.

```powershell
# Build (CGO_ENABLED=0, -H windowsgui)
pwsh ./scripts/agent-run.ps1 build

# Run unit tests
go test ./... -v

# Verification harness (Phase 4)
pwsh ./scripts/agent-run.ps1 start --agent-mode
pwsh ./scripts/agent-run.ps1 status
pwsh ./scripts/agent-run.ps1 logs
pwsh ./scripts/agent-run.ps1 kill
```

## Backlog / Deferred Features

Features explicitly deferred past v1.0 — in priority order:

- **F29 — Code Signing (Azure Artifact Signing, Public Trust).** Release
  workflow already wires `signtool` conditionally behind the
  `AZURE_TRUSTED_SIGNING_ENDPOINT` secret. Deferred to v1.1+ to amortize
  the per-signature cost against Scoop install count > ~50.
- **F30 — Winget integration.** Deferred until Scoop install count
  justifies the additional submission + review overhead.
- **F31 — User-configurable filename template.** v1.0 hardcodes
  `YYYY-MM-DD_HH-MM-SS_mmm.png` (collision-free via ms precision). A
  template engine introduces parsing/validation complexity not needed
  for v1.0 scope.
- **F32 — `@`-prefix clipboard mode for Claude Code attachments.** Optional
  config flag to write `@<path>` instead of a quoted path. Deferred
  until Claude Code's `@path` convention stabilizes.
- **F33 — In-overlay annotation / drawing.** Shapes / arrows / text
  overlay before save. Substantial UI surface; outside v1.0 scope.
- **F34 — OCR (text extraction from captured image).** Requires a vision
  dependency (tesseract / cloud API). Scope creep.
- **F35 — Direct WinRT toast fallback.** `github.com/go-toast/toast` is
  frozen since 2019; if it breaks on a future Windows build, a
  `ToastNotificationManager` WinRT wrapper (~150 LOC) is the documented
  crisis fallback. Not implemented preemptively.
- **F36 — Architecture doc reconciliation.** The `event=subsystem.error`
  wording in `architecture.md` §Cross-cutting Patterns predates the
  enumerated per-subsystem `*.error` events in §Established Decisions.
  A v1.1+ doc revision will reconcile both sections.
- **F37 — Settings UI dialog.** Phase 2's tray menu ships a grayed
  `Settings (edit config.toml)` placeholder by design (AC #3 — menu
  layout stability). The actual modal (file picker, hotkey capture,
  `config.toml` round-trip) is deferred to v1.1+. Until then, users
  edit `%APPDATA%\clip-clap\config.toml` directly.

## License

Apache 2.0 — see [LICENSE](LICENSE).
