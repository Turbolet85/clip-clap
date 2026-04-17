# Clip Clap

Minimalist Windows tray utility: hotkey-triggered area-select **screenshot**,
saves PNG to a configured folder, swaps the clipboard to the auto-quoted
absolute path so it can be pasted directly into a console (Windows Terminal,
VS Code, WSL, SSH).

**Status:** Phase 0 (foundation skeleton). Not yet feature-complete; v1.0
ships when Phases 1–5 land.

## Installation

Download `clip-clap.exe` from
[GitHub Releases](https://github.com/Turbolet85/clip-clap/releases)
(once v1.0 is tagged), or install via Scoop:

```powershell
scoop bucket add Turbolet85 https://github.com/Turbolet85/scoop-bucket
scoop install clip-clap
```

v1.0 ships unsigned — Windows SmartScreen will warn on first run. Click
"More info" → "Run anyway". v1.1+ will ship signed via Azure Artifact
Signing.

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

## License

Apache 2.0 — see [LICENSE](LICENSE).
