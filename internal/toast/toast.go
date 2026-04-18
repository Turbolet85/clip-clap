// Package toast owns Windows toast-notification dispatch via the upstream
// go-toast/toast library. See Phase 3 plan Step 12 and architecture.md
// §[Toast Notification Library] for the canonical contract.
package toast

import (
	"fmt"
	"log/slog"
	"path/filepath"

	gotoast "github.com/go-toast/toast"

	"github.com/Turbolet85/clip-clap/internal/lasterror"
	"github.com/Turbolet85/clip-clap/internal/logger"
	"github.com/Turbolet85/clip-clap/internal/tray"
)

// notificationPusher abstracts gotoast.Notification.Push() for testing.
// Production pushes via the real library; tests substitute a stub that
// records invocations.
type notificationPusher func(n *gotoast.Notification) error

var push notificationPusher = func(n *gotoast.Notification) error { return n.Push() }

// SetPusherForTesting injects a custom pusher for unit tests.
func SetPusherForTesting(p notificationPusher) { push = p }

// ResetPusher restores the production pusher.
func ResetPusher() {
	push = func(n *gotoast.Notification) error { return n.Push() }
}

// Show displays a Windows toast notification for a completed capture. The
// body reads "Captured: <filename>" where <filename> is the basename only
// (Windows backslashes preserved by design — see architecture.md).
//
// On success: emits `toast.shown` event with capture_id, returns nil.
// On failure: emits `toast.error` event, sets lasterror via SanitizeForTray,
// returns the error (non-fatal — caller continues the pipeline; the tray
// icon + tooltip remain the primary receipt per design-system).
func Show(absPath, captureID, saveFolder string) error {
	filename := filepath.Base(absPath)
	n := &gotoast.Notification{
		AppID:   DefaultAppID,
		Title:   "Clip Clap",
		Message: fmt.Sprintf("Captured: %s", filename),
	}
	if saveFolder != "" {
		n.Actions = []gotoast.Action{
			{Type: "protocol", Label: "Show", Arguments: saveFolder},
		}
	}

	if err := push(n); err != nil {
		slog.Error("toast failed",
			"event", logger.EventToastError,
			"capture_id", captureID,
			"error", err.Error(),
		)
		lasterror.Set(tray.SanitizeForTray(err))
		return fmt.Errorf("toast show: %w", err)
	}

	slog.Info("toast shown",
		"event", logger.EventToastShown,
		"capture_id", captureID,
	)
	return nil
}
