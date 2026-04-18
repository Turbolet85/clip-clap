"""Phase 3 toast-notification test — activated in Phase 4."""
import pytest

pytestmark = pytest.mark.skip(
    reason="Phase 3 toast test requires desktop session + UIA drag"
)


def test_toast_shown_after_capture(agent):
    """Capture → logs contain toast.shown event with matching capture_id."""
    pass
