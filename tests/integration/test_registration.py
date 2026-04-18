"""Phase 3 hotkey registration tests — activated in Phase 4."""
import pytest

pytestmark = pytest.mark.skip(
    reason="Phase 3 registration tests rely on desktop session + UIA"
)


def test_hotkey_registered_event_logged(agent):
    """Assert hotkey.registered event in logs after agent start."""
    pass
