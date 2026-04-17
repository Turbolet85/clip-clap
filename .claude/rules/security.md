# Security Rules

Universal security requirements. Apply to all files in this project. This rule file has no `paths:` frontmatter — it loads unconditionally.

## Secrets
- Never commit files matching `.env*` patterns
- No production credentials are in scope: this is a single-user local tool with no accounts and no remote services
- The only "identity" is the Windows named mutex `Global\ClipClapSingleInstance` and AppUserModelID `Turbolet85.ClipClap` — neither is a secret
- If signing certificates are introduced (v1.1+ Azure Trusted Signing), thumbprints and credentials live in GitHub Actions secrets, never in source

## Input validation
- **Config file (TOML)** — strict unmarshal mode is mandatory: unknown keys must be rejected at parse time so typos surface immediately as `config.error` events
- **Environment variables** (`CLIP_CLAP_CONFIG`, `CLIP_CLAP_SAVE_DIR`, `CLIP_CLAP_LOG_PATH`, `CLIP_CLAP_DEBUG`) — treat as untrusted user input even though the user controls them
- **Save folder paths** — validate that the resolved path is absolute and writable before the first capture. Auto-create missing directories with `os.MkdirAll(path, 0o755)` (idempotent). Refuse to write outside the configured save folder
- **Filename construction** — never interpolate user input into filenames. Format is hardcoded `YYYY-MM-DD_HH-MM-SS_mmm.png`; no template engine in v1
- **Status endpoint** — loopback-only (`127.0.0.1:27773`), `--agent-mode`-gated (off by default). Reject any request from a non-loopback origin at the listener level. Never add auth, never proxy, never expose externally

## Clipboard handling
- The clipboard swap is **destructive** — always snapshot the prior content into the in-memory Undo buffer BEFORE writing
- The 500ms clipboard reentry guard is per-`capture_id` — never widen the window or make it global; doing so silently drops legitimate external clipboard writes
- Never log clipboard contents (prior or new) — the path itself goes into structured logs as `path` field, but no other clipboard text is captured
- Never log the clipboard-undo prior content — it may contain anything the user copied (passwords, PII)

## Logging discipline (privacy)
- Structured JSON only via `log/slog` JSONHandler — never `fmt.Println` or `log.Printf` (those bypass the redaction discipline)
- Allowed log fields: `event`, `capture_id`, `path` (saved file path only), `error`, `timestamp`, `hotkey`, `config_path`, `save_folder`, `existing_pid`
- Never log: clipboard contents, prior clipboard contents, image bytes, user-typed content, full environment variable dumps
- See `.claude/rules/observability.md` for the complete event enum

## Resource handling
- **Win32 handles** — every `OpenClipboard`, `CreateWindowExW`, `RegisterHotKey`, `CreateMutex` must have a paired `Close*` / `UnregisterHotKey` / `ReleaseMutex` via `defer`
- **Goroutines** — every subsystem goroutine must respect a context cancel; no orphaned goroutines on shutdown
- **Files** — `defer f.Close()` immediately after open; PNG writes use `os.WriteFile` (atomic via temp + rename if extending)

## Build & dependency hygiene
- **`CGO_ENABLED=0` is mandatory** — CI fails loudly if it leaks to `1`. Reason: cross-compilation, Defender false-positives, MinGW dependency removal
- **`go.mod` pins exact versions** (no ranges) — see architecture.md `[Go Module Version Pinning]`. `go.sum` is committed
- **`govulncheck`** — run `govulncheck ./...` before each release (manual until automated in CI). Patch any reported CVEs before tagging
- Review major version bumps for breaking changes; never blindly accept Dependabot upgrades on dependencies that touch Win32, clipboard, or screenshot capture

## Code signing
- v1.0: unsigned (accepted SmartScreen warning trade-off)
- v1.1+: Azure Trusted Signing via GitHub Actions secrets. Cert thumbprint stored in `secrets.AZURE_SIGN_THUMBPRINT`, never in source. `signtool sign` runs only when the secret is present (graceful no-op otherwise)

## Session Additions
_This section is owned by `/wrap-session`. setup-project preserves content added here on re-run. See `section-markers.md` for the convention._
