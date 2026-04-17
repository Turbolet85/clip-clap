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
	EventHotKeyRegistered        = "hotkey.registered"
	EventHotKeyError             = "hotkey.error"
	EventTrayMenuOpened          = "tray.menu.opened"
	EventConfigLoaded            = "config.loaded"
	EventConfigError             = "config.error"
	EventSingleInstanceViolation = "single_instance.violation"
)
