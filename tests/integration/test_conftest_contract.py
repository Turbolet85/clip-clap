"""Meta-test for the conftest agent fixture: did it yield only after ready=true?"""
import threading
import time

import pytest
import requests

from conftest import STATUS_URL

# Shared mutable state for the side-channel polling thread. Guarded by
# the GIL — no explicit lock needed for read-modify-write of simple vars.
_first_ready_true_at: float = 0.0
_watcher_stop = threading.Event()


def _watcher():
    """Background poller that records the timestamp of the first
    response with ready=true. Runs until the main thread signals stop.
    """
    global _first_ready_true_at
    while not _watcher_stop.is_set():
        try:
            r = requests.get(STATUS_URL, timeout=0.2)
            if r.status_code == 200 and r.json().get("ready") is True:
                if _first_ready_true_at == 0.0:
                    _first_ready_true_at = time.monotonic()
                    return
        except (requests.ConnectionError, requests.Timeout):
            pass
        time.sleep(0.02)


@pytest.mark.skip(
    reason=(
        "Requires agent fixture to run concurrently with the watcher; "
        "current fixture design is blocking. Meta-test activated "
        "when the fixture is refactored for overlapping runs."
    )
)
def test_fixture_waits_for_ready(agent):
    """Spawn a side-channel thread before the fixture yields; assert
    the fixture's yield time is AFTER the first observed ready=true.
    """
    # Placeholder — the side-channel approach needs a threaded fixture
    # design; current fixture blocks before yielding. Keeping as a
    # documented future test.
    pass
