package tray

// Menu item IDs for the TrackPopupMenuEx context menu. The WndProc in
// cmd/clip-clap/main.go dispatches WM_COMMAND messages by extracting the
// low 16 bits of wparam and matching against these constants. Keep the
// values small and contiguous so Win32's 16-bit wparam range is never
// exhausted, and so static analysis can see the dispatch is dense.
//
// The design-system labels associated with each ID are defined in
// ShowContextMenu (tray.go), not here, because this file must stay pure
// data for Step 3's single-file unit test pass. Adding label strings
// here would pull in design-token prose and muddy the test surface.
const (
	MenuIDCapture         = 1 // "Expose\tCtrl+Shift+S" — darkroom verb for the capture action (NOT "Capture")
	MenuIDOpenFolder      = 2 // "Open folder" — explorer.exe to cfg.SaveFolder
	MenuIDSettings        = 3 // [v1.0.7 removed from visible menu] was "Settings (edit config.toml)" grayed
	MenuIDQuit            = 4 // "Quit" — posts WM_CLOSE to the message pump
	MenuIDUndoLastCapture = 5 // [v1.0.7 removed from visible menu] was "Undo last capture"
	MenuIDLastError       = 6 // [v1.0.7 removed from visible menu] was "Last error: <none>"
	MenuIDEditHotkey      = 7 // "Edit hotkey (restart to apply)" — opens config.toml in default editor
)

// MenuIDToName returns a human-readable identifier for logging and tests.
// Not a user-facing label — the design system's "Expose"/"Open folder"
// strings live in ShowContextMenu. This helper is for developer-facing
// surfaces only (slog attributes, test output, panic messages).
func MenuIDToName(id int) string {
	switch id {
	case MenuIDCapture:
		return "capture"
	case MenuIDOpenFolder:
		return "open_folder"
	case MenuIDSettings:
		return "settings"
	case MenuIDQuit:
		return "quit"
	case MenuIDUndoLastCapture:
		return "undo_last_capture"
	case MenuIDLastError:
		return "last_error"
	case MenuIDEditHotkey:
		return "edit_hotkey"
	default:
		return "unknown"
	}
}
