# clip-clap

**Windows screenshot tool that puts the file path in your clipboard, not the image.**

Press a hotkey → drag an area → clip-clap saves a PNG and copies the
absolute file path to the clipboard. Paste that path straight into
Windows Terminal, VS Code, WSL, SSH, Claude Code, ChatGPT, or any chat —
the consumer gets a real file reference, not a bitmap it can't use.

---

## Why?

Windows already has `Win+Shift+S` (Snipping Tool) for area-screenshots.
But Snipping Tool puts the **image** on the clipboard, which is useless
if you want to:

- Paste a screenshot into a terminal so an AI coding assistant can read
  the file directly from disk (e.g. `claude: look at @<paste>`)
- Attach a screenshot to a chat that expects a file path, not a bitmap
- Reference the saved file from a script or command

clip-clap solves exactly that one problem — **hotkey → PNG saved to
disk → file path on the clipboard** — and nothing else. No editing
tools, no annotations, no cloud sync, no accounts.

## Features

- **Area-select screenshot** via transparent full-screen overlay (works
  across all monitors, any DPI)
- **PNG file saved** to `%USERPROFILE%\Pictures\clip-clap\` with a
  millisecond-precision filename (`2026-04-18_21-27-49_321.png`)
- **Absolute path copied** to the clipboard, auto-quoted if it contains
  spaces (`"C:\Program Files\...\shot.png"`)
- **Windows toast notification** confirms each capture with the filename
- **Structured JSON logs** (one event per capture, parseable via `jq`)
- **Configurable hotkey** via `config.toml`
- **Single static 8 MB `.exe`** — no runtime, no installer, no admin
- **Pure Go + Win32** — no CGO, no MinGW

## Install

### Scoop (recommended)

```powershell
scoop bucket add clip-clap https://github.com/Turbolet85/scoop-bucket
scoop install clip-clap
```

### Direct download

Grab `clip-clap.exe` from the
[latest release](https://github.com/Turbolet85/clip-clap/releases/latest)
and drop it anywhere (Desktop, `%USERPROFILE%\tools\`, wherever).

> **First run will show a SmartScreen warning** — v1.0 is unsigned.
> Click **More info** → **Run anyway** once and Windows remembers
> your decision. Code signing is planned for v1.1+.

## Usage

1. **Double-click `clip-clap.exe`** — the pentagon tray icon appears
   in the notification area.
2. **Press `Ctrl+Shift+S`** (default) from any app → the screen dims
   and a crosshair appears.
3. **Drag a rectangle** with the left mouse button → the overlay
   disappears, a PNG file is saved, the file path is copied to your
   clipboard, and a Windows toast confirms the capture.
4. **Paste (`Ctrl+V`) anywhere** — terminal, chat, editor — and you
   get the path. The image file is already on disk for anyone (or
   anything) to open.

**Press `Esc`** on the overlay to cancel without capturing.

### Tray menu (right-click the tray icon)

- **Expose** — fire a capture without using the hotkey
- **Open folder** — opens the save folder in Explorer
- **Edit hotkey** — opens `config.toml` in your default TOML editor
- **Quit** — closes the app

## Configuration

Config lives at `%USERPROFILE%\Pictures\clip-clap\config.toml` and
auto-generates on first run with annotated defaults. The `Edit hotkey`
tray menu item opens it for you.

```toml
save_folder = "C:\\Users\\you\\Pictures\\clip-clap\\"
hotkey = "Ctrl+Shift+S"
auto_quote_paths = true
log_level = "INFO"
```

**Hotkey format** — **case-sensitive**, capitalize only the first letter:

- Modifiers: `Ctrl`, `Shift`, `Alt`, `Win`
- Keys: `A`–`Z`, `0`–`9`, `F1`–`F12`, `Space`, `PageUp`, `PageDown`,
  `Home`, `End`, `Insert`, `Delete`
- Combine with `+`: `Alt+F12`, `Win+V`, `Ctrl+Shift+PageUp`

Invalid values (`ctrl+s`, `CTRL+S`, `Ctrl+Home+End`) are rejected at
startup; clip-clap logs `hotkey.error` and runs without an active
hotkey (tray `Expose` still works so you can fix the config).

**Environment variable overrides:**

- `CLIP_CLAP_CONFIG` — alternate `config.toml` path (absolute)
- `CLIP_CLAP_SAVE_DIR` — override save folder (absolute)
- `CLIP_CLAP_LOG_PATH` — redirect log file (absolute)
- `CLIP_CLAP_DEBUG=1` — bump log level to DEBUG

## Troubleshooting

**Hotkey doesn't fire.** Another app is probably holding the same
chord (Discord, OBS, Photoshop, browser extensions). Check
`logs\agent-latest.jsonl` for `hotkey.error`, then pick a different
combo in `config.toml` (tray → Edit hotkey → change → Quit → relaunch).

**Desktop icon looks wrong after updating.** Windows caches icons
aggressively. Delete `clip-clap.exe` from Desktop, empty Recycle Bin,
redownload. Icon refreshes.

**Log and config can't be found.** They live next to your screenshots:
`%USERPROFILE%\Pictures\clip-clap\config.toml` and
`%USERPROFILE%\Pictures\clip-clap\logs\agent-latest.jsonl`.

## Under the hood

- Pure Go 1.23+, `CGO_ENABLED=0`, direct Win32 via
  `golang.org/x/sys/windows`
- No dependencies outside `go.sum`
- Shared Win32 message pump handles hotkey + tray + overlay
- `RegisterHotKey` on thread-locked goroutine (`runtime.LockOSThread`)
- Structured logging via stdlib `log/slog` JSON
- `CreateMutex(Local\ClipClapSingleInstance)` for single-instance
- Designed and scaffolded by the [Andromeda](https://github.com/Turbolet85/andromeda)
  pipeline

## Build from source

```powershell
git clone https://github.com/Turbolet85/clip-clap
cd clip-clap
pwsh ./scripts/agent-run.ps1 build
# Output: ./clip-clap.exe (~8 MB, CGO_ENABLED=0, -H windowsgui)
```

Requires Go 1.23+ and `goversioninfo` for the resource embed:
```powershell
go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@v1.5.0
```

## Roadmap

**v1.0.x** (current, multiple bug-fix patches): core capture flow,
tray, hotkey, toast, configurable hotkey.

**v1.1+** candidates (no firm dates):

- Code signing (Azure Artifact Signing, Public Trust) — will suppress
  SmartScreen warning
- In-app hotkey rebind modal (no-restart)
- `@path` prefix mode for Claude Code attachments
- Winget submission
- Direct WinRT toast fallback (if `go-toast/toast` breaks)

See [CHANGELOG.md](CHANGELOG.md) for the detailed release notes.

## License

[Apache 2.0](LICENSE) — use it, fork it, ship it.
