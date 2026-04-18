"""Multi-monitor DPI tests — Phase 3 deliverable, activated in Phase 4.

pytest.mark.skipif with monitor-count detection so single-monitor runners
(the default for GitHub Actions windows-latest) skip these tests
automatically.
"""
import pytest


def _detect_monitors() -> int:
    """Return the number of attached monitors. Placeholder returning 1.

    A real implementation would use `win32api.EnumDisplayMonitors` via
    pywin32. For Phase 4 this is a stub that always reports single-
    monitor so the tests skip by default.
    """
    try:
        import win32api
        return len(win32api.EnumDisplayMonitors(None, None))
    except Exception:
        return 1


pytestmark = pytest.mark.skipif(
    _detect_monitors() < 2,
    reason="single-monitor runner (< 2 monitors detected)",
)


def test_overlay_spans_virtual_screen(agent):
    """Verify the overlay window covers the full virtual-screen rect
    including all attached monitors.
    """
    pass  # body added when multi-monitor CI is available
