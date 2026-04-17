# Commands Reference

_Complete command reference extracted from `.andromeda/architecture.md` + project setup by `/setup-project`. The 3-5 most common commands live in CLAUDE.md Workflow section for always-loaded access — this file has everything else._

## Module setup (one-time)
- `go mod init github.com/Turbolet85/clip-clap` — initialize the module (only on first scaffold)
- `go mod tidy` — sync `go.mod` / `go.sum` after dependency changes
- `go mod download` — fetch dependencies into the local module cache

## Build
- `go build -ldflags="-H windowsgui -s -w" -o clip-clap.exe ./cmd/clip-clap` — production build (CGO_ENABLED=0 enforced; `-H windowsgui` hides console; `-s -w` strips symbols)
- `go build -o clip-clap-debug.exe ./cmd/clip-clap` — debug build (console visible, symbols intact)
- `go generate ./...` — runs `goversioninfo` to produce `resource.syso` (icon + manifest embedded into the next `go build`)

## Run
- `.\clip-clap.exe` — normal user mode (tray icon, hotkey active, status endpoint disabled)
- `.\clip-clap.exe --agent-mode` — verification mode (status endpoint on `127.0.0.1:27773` enabled, PID written to `.agent-running`)
- `.\clip-clap.exe --debug` — bumps log level to DEBUG (equivalent to `CLIP_CLAP_DEBUG=1`)

## Verification harness (PowerShell)
- `pwsh ./scripts/agent-run.ps1 build` — compile the agent
- `pwsh ./scripts/agent-run.ps1 start [--agent-mode]` — launch the agent, write PID to `.agent-running`, redirect logs to `logs/agent-latest.jsonl`
- `pwsh ./scripts/agent-run.ps1 status` — `curl http://localhost:27773/status` (returns JSON)
- `pwsh ./scripts/agent-run.ps1 logs` — print `logs/agent-latest.jsonl`
- `pwsh ./scripts/agent-run.ps1 kill` — read PID from `.agent-running`, send `WM_CLOSE` (or `taskkill /PID`)

## Testing
- `go test ./... -cover` — all unit tests with coverage report
- `go test ./internal/capture -run TestFormatFilename -v` — single test, single package, verbose
- `go test ./... -race` — run with race detector (use before merging anything goroutine-related)
- `pytest tests/integration/ -v` — full integration suite (requires Windows + UIA + built `.exe`)
- `pytest tests/integration/test_clipboard.py -v` — single integration test file
- `pytest tests/integration/test_clipboard.py::test_clipboard_swap_with_spaces -v` — single test function

## Linting & formatting
- `gofmt -w .` — format all Go files in-place (also wired into Claude Code PostToolUse hook)
- `gofmt -l .` — list files that would be reformatted (no writes) — useful in pre-merge check
- `go vet ./...` — built-in static checks (always run before commit)
- `golangci-lint run --fix ./...` — meta-linter with auto-fix (optional; not installed by default — `scoop install golangci-lint` to enable)

## Security & dependency hygiene
- `govulncheck ./...` — scan for known vulnerabilities in dependencies (run before each release tag)
- `go list -m -u all` — list outdated dependencies
- `go mod tidy && git diff go.sum` — verify `go.sum` doesn't drift unexpectedly

## Git & release
- **Branch naming:** `feature/{description}`, `fix/{description}`, `chore/{description}`
- **Conventional commits:** `feat:`, `fix:`, `chore:`, `refactor:`, `docs:`, `test:`, `perf:`, `build:`, `ci:`
- `git tag v1.0.0` followed by `git push --tags` — triggers GitHub Actions `build-release` + `publish-release` jobs
- Never `git push --force` to `main`

## Scoop / distribution
- `scoop bucket add <user>/<bucket>` — register the personal bucket containing `clip-clap.json`
- `scoop install clip-clap` — install from the bucket
- Manifest is updated in the bucket repo on each release with new version + SHA256

## Troubleshooting
- `taskkill /PID $(cat .agent-running)` — manually kill an orphaned `--agent-mode` process
- `Remove-Item .agent-running` — clean up stale PID file (only if no live process holds it)
- `Get-Content logs/agent-latest.jsonl -Wait` — tail the JSON log
- `curl http://127.0.0.1:27773/status` — sanity-check the status endpoint when troubleshooting integration tests
- `dotnet --info` / `python --version` / `go version` — confirm toolchain availability

## Environment variables
- `CLIP_CLAP_CONFIG` — full path to alternative `config.toml`
- `CLIP_CLAP_SAVE_DIR` — absolute Windows path; overrides `save_folder` from config
- `CLIP_CLAP_DEBUG=1` — bumps log level to DEBUG
- `CLIP_CLAP_LOG_PATH` — absolute path; redirects log output
- `CGO_ENABLED=0` — enforced in CI; never set to `1`
