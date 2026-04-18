// Package logger owns structured logging via log/slog with event enums and RFC 3339 nanosecond timestamps.
package logger

// Event name constants matching the full enum from architecture §Structured
// Log Event Schema. Using a single `const (...)` block is idiomatic Go; each
// constant's string value is the canonical wire-format event name used by
// integration tests, status-endpoint consumers, and log-parsing harnesses.
const (
	EventCaptureStarted          = "capture.started"
	EventCaptureCompleted        = "capture.completed"
	EventCaptureFailed           = "capture.failed"
	EventClipboardSwap           = "clipboard.swap"
	EventClipboardUndo           = "clipboard.undo"
	EventToastShown              = "toast.shown"
	EventToastError              = "toast.error"
	EventTrayFlashError          = "tray.flash.error"
	EventHotKeyRegistered        = "hotkey.registered"
	EventHotKeyError             = "hotkey.error"
	EventTrayMenuOpened          = "tray.menu.opened"
	EventConfigLoaded            = "config.loaded"
	EventConfigError             = "config.error"
	EventSingleInstanceViolation = "single_instance.violation"
	// EventAgentDisabled is emitted when the agent would otherwise
	// respond to an environment-driven configuration (e.g., a future
	// CLIP_CLAP_AGENT_PORT) but --agent-mode is not set. Reserved for
	// Phase 4+ multi-port / alternative-port scenarios; Phase 4 itself
	// emits no events with this constant (the CLIP_CLAP_AGENT_PORT
	// check was dropped in Phase 5 validation as out-of-scope).
	EventAgentDisabled = "agent.disabled"
)
