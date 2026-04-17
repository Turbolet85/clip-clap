---
paths:
  - "internal/log/**"
  - "internal/status/**"
  - "**/*_logger.go"
  - "**/*_events.go"
---

# Observability Rules

Path-scoped rules for logging and status-endpoint code. The project intentionally has **no** OpenTelemetry, no metrics, no distributed tracing — the verification profile is `desktop-windows`, not server. The single observability surface is structured JSON logs + a loopback status endpoint.

## Logging
- **Library:** Go stdlib `log/slog` with `slog.NewJSONHandler` only — never `fmt.Println`, `log.Printf`, or third-party loggers
- **Output:** `logs/agent-latest.jsonl` (one JSON object per line) — path overridable via `CLIP_CLAP_LOG_PATH` env var
- **Default level:** `INFO`; `--debug` flag or `CLIP_CLAP_DEBUG=1` env var bumps to `DEBUG`
- **Rotation:** out of scope. No internal rotation; manual rotation or external `logrotate`-equivalent if ever needed
- **Mandatory fields per entry:** `event` (string from the enum below), `timestamp` (RFC 3339 nanosecond precision)
- **Per-event optional fields:** see `internal/log/events.go` event enum and the per-event field list in `architecture.md` `[Structured Log Event Schema]`

## Event enum (verification contract)
The complete enum lives in `internal/log/events.go`. Tests assert on these exact strings; do **not** rename or remove an event without updating both the enum and every integration test that asserts on it:

- `capture.started`, `capture.completed`, `capture.failed`
- `clipboard.swap`, `clipboard.undo`
- `toast.shown`, `toast.error`
- `hotkey.registered`, `hotkey.error`
- `tray.menu.opened`
- `config.loaded`, `config.error`
- `single_instance.violation`

Adding a new event:
1. Add the constant to `internal/log/events.go`
2. Document the event + optional fields in `architecture.md` `[Structured Logging Keys (Verification Contract)]` AND `[Structured Log Event Schema]`
3. Add at least one pytest assertion for the new event in `tests/integration/`

## Privacy
- Never log clipboard contents (current or prior) — the saved file `path` is allowed; raw clipboard text is never logged
- Never log image bytes, screen contents, or pixel data
- Never log full environment variable dumps; log only the specific resolved values that affect behavior (e.g., `config_path`, `save_folder`)
- `clipboard.undo` event has no `path` field by design — undo restores arbitrary prior content that may be sensitive

## ID conventions
- `capture_id` is a ULID via `github.com/oklog/ulid/v2` — lexically sortable, embeds millisecond timestamp, collision-free without sync
- One `capture_id` is generated per capture flow and propagated across every event in that flow (`capture.started` → `capture.completed`/`.failed` → `clipboard.swap` → `toast.shown`/`.error`)
- `capture_id` is in-memory only; never persisted (no database in scope)

## Error events
- `*.error` and `*.failed` events MUST include an `error` field with the underlying error message (`fmt.Errorf("...: %w", err).Error()`)
- Wrap errors with `fmt.Errorf("context: %w", err)` so `errors.Is` and `errors.As` work upstream
- Surface fatal subsystem errors via `slog` with `event=<subsystem>.error` BEFORE the goroutine exits — the tray menu's "Last error" entry mirrors the latest such event

## Status endpoint
- `GET http://127.0.0.1:27773/status` returns flat JSON: `{"ready": bool, "last_capture": "filename.png", "pid": int, "version": "x.y.z"}`
- **Loopback bind only** — `127.0.0.1`, never `0.0.0.0`. Reject non-loopback origins at the listener level
- **`--agent-mode` gated** — endpoint is OFF by default; only the test harness runs the agent with this flag
- HTTP semantics: `200` with `ready=true|false`, `503` if server crashed, `404` if endpoint disabled
- Never add auth, never add additional endpoints, never proxy — this is a test hook, not a product API

## Tracing & metrics
- **N/A by design.** No OTel SDK, no Prometheus client, no metrics exporter. If a future scope ever needs them, add via `/andromeda-sigma` first

## Session Additions
_This section is owned by `/wrap-session`. setup-project preserves content added here on re-run._
