"""Phase 3 hotkey + overlay integration tests — activated in Phase 4.

Stubbed: Phase 4 creates these files so CI's pytest collection picks
them up; the actual test bodies require UIA + desktop session, which
may not be available in every CI environment. Skipped via marker.
"""
import pytest

pytestmark = pytest.mark.skip(
    reason=(
        "Phase 3 overlay tests require interactive desktop + UIA; "
        "implementer should expand these with real UIA drag once "
        "GitHub Actions windows-latest + UIA availability is confirmed."
    )
)


def test_hotkey_creates_overlay(agent):
    """Press Ctrl+Shift+S, assert overlay window appears."""
    pass  # see pytestmark


def test_overlay_drag_captures_rect(agent):
    """Drag overlay rect, assert PNG saved, overlay dismissed."""
    pass


def test_capture_events_logged(agent):
    """Assert capture.started + capture.completed in logs."""
    pass
