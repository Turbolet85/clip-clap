"""Phase 3 clipboard integration tests — activated in Phase 4."""
import pytest

pytestmark = pytest.mark.skip(
    reason="Phase 3 clipboard tests require desktop session + UIA drag"
)


def test_clipboard_swap_with_spaces(agent):
    """Capture → clipboard contains auto-quoted absolute path."""
    pass


def test_clipboard_swap_no_quote_config(agent):
    """With auto_quote_paths=false, clipboard has unquoted path."""
    pass


def test_clipboard_undo(agent):
    """Pre-set clipboard → capture → Undo → prior content restored."""
    pass
