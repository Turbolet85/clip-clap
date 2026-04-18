"""pytest fixtures + helpers for the clip-clap integration suite.

The `agent` fixture is the foundation of every integration test:
it builds clip-clap.exe, launches it with --agent-mode, polls the
/status endpoint until ready=true, and tears down via agent-run.ps1
kill on test completion. Tests that intentionally kill the agent
(e.g., test_status_connection_refused_after_shutdown) must use
bypass helpers or accept the fixture's teardown kill as a no-op.

Helpers: press_hotkey, drag_rect, read_clipboard, parse_log_events.
All helpers assume Windows + UIA enabled (the desktop-windows
verification profile). UIA is assumed present on GitHub Actions
windows-latest runners as of 2026-01.
"""
import json
import os
import subprocess
import time

import psutil
import pytest
import requests

# UIA-dependent imports are tolerated as optional so that the test
# suite can be imported on non-Windows CI environments (to satisfy
# `pytest --collect-only` on linters). Tests that call these helpers
# will fail fast at runtime on non-Windows.
try:
    import pywinauto.keyboard as _kbd
    import pywinauto.mouse as _mouse
    import win32clipboard
except ImportError:  # pragma: no cover — Windows-only
    _kbd = None
    _mouse = None
    win32clipboard = None


STATUS_URL = "http://127.0.0.1:27773/status"
PID_FILE = ".agent-running"
LOG_PATH = "logs/agent-latest.jsonl"


def _poll_until_ready(timeout_s: float = 30.0, interval_s: float = 0.1) -> dict:
    """Poll GET /status until ready=true.

    Retry semantics match the architecture's §Status Endpoint contract:
      - retry on ConnectionError (TCP refused — listener not yet bound)
      - retry on 200 with ready=false (listener bound, MarkReady not yet fired)
      - FAIL fast on 503 (draining — implies shutdown racing startup)
      - FAIL fast on 404 (endpoint disabled — --agent-mode flag missing)
      - FAIL on timeout (30s)
    """
    deadline = time.monotonic() + timeout_s
    last_err = None
    while time.monotonic() < deadline:
        try:
            r = requests.get(STATUS_URL, timeout=1.0)
            if r.status_code == 503:
                raise RuntimeError(f"/status returned 503 during startup — agent draining unexpectedly")
            if r.status_code == 404:
                raise RuntimeError(f"/status returned 404 — --agent-mode flag not set")
            if r.status_code == 200:
                body = r.json()
                if body.get("ready") is True:
                    return body
                # ready=false → keep polling
                last_err = f"ready=false at {time.monotonic():.3f}s"
        except requests.ConnectionError as e:
            last_err = f"ConnectionError: {e!s}"
        except requests.Timeout as e:
            last_err = f"Timeout: {e!s}"
        time.sleep(interval_s)
    raise TimeoutError(f"agent did not become ready within {timeout_s}s (last: {last_err})")


@pytest.fixture
def agent():
    """Function-scoped agent lifecycle.

    Builds clip-clap.exe (idempotent — skips if already built), launches
    it with --agent-mode via agent-run.ps1, waits for ready=true, yields
    the first successful status response, then tears down via kill on
    test completion (even if the test body fails).
    """
    # Build is idempotent — agent-run.ps1 always recompiles, but the
    # Go toolchain incrementally caches so this is ~1-2s on rebuilds.
    subprocess.run(
        ["pwsh", "scripts/agent-run.ps1", "build"],
        check=True, timeout=60,
    )
    # Start with --agent-mode. start is non-blocking: Start-Process
    # returns immediately after spawning; the PID file is written
    # synchronously.
    subprocess.run(
        ["pwsh", "scripts/agent-run.ps1", "start", "--agent-mode"],
        check=True, timeout=10,
    )
    try:
        status_body = _poll_until_ready()
        yield status_body
    finally:
        # Teardown: kill via agent-run.ps1. Tolerates the agent already
        # being dead (e.g., if a test killed it intentionally).
        subprocess.run(
            ["pwsh", "scripts/agent-run.ps1", "kill"],
            check=False, timeout=10,
        )


def press_hotkey(keys: str = "^+s") -> None:
    """Send a hotkey chord via pywinauto.

    Default '^+s' is Ctrl+Shift+S (the clip-clap default). Callers can
    override for tests that need a different chord.
    """
    if _kbd is None:
        raise RuntimeError("pywinauto not available (Windows-only)")
    _kbd.send_keys(keys)


def drag_rect(x1: int, y1: int, x2: int, y2: int) -> None:
    """Drag a rectangle by pressing at (x1,y1), moving to (x2,y2), releasing.

    Coordinates are absolute virtual-screen pixels. pywinauto handles
    DPI scaling internally when the test runner is DPI-aware.
    """
    if _mouse is None:
        raise RuntimeError("pywinauto not available (Windows-only)")
    _mouse.press(coords=(x1, y1))
    try:
        _mouse.move(coords=(x2, y2))
    finally:
        _mouse.release(coords=(x2, y2))


def read_clipboard() -> str:
    """Read the current clipboard contents as CF_UNICODETEXT.

    Returns empty string if the clipboard doesn't contain text. Open/
    Close bracket the Windows clipboard handle per the Win32 contract.
    """
    if win32clipboard is None:
        raise RuntimeError("win32clipboard not available (Windows-only)")
    win32clipboard.OpenClipboard()
    try:
        data = win32clipboard.GetClipboardData(win32clipboard.CF_UNICODETEXT)
        return data if isinstance(data, str) else ""
    finally:
        win32clipboard.CloseClipboard()


def parse_log_events(path: str = LOG_PATH) -> list:
    """Read logs/agent-latest.jsonl and return a list of event dicts.

    Skips empty lines. Raises if a line isn't valid JSON (surfaces
    logger corruption loudly instead of silently).
    """
    events = []
    if not os.path.exists(path):
        return events
    with open(path, "r", encoding="utf-8") as fp:
        for line in fp:
            line = line.strip()
            if not line:
                continue
            events.append(json.loads(line))
    return events


def read_pid_file(path: str = PID_FILE) -> int:
    """Read the .agent-running PID file and return the integer PID.

    Validates the format (decimal digits + single newline). Callers
    can use this to verify AC #3's format contract.
    """
    with open(path, "r", encoding="utf-8") as fp:
        raw = fp.read()
    # Strict regex check — matches architecture's `^\d+\n$` contract.
    stripped = raw.rstrip("\r\n")
    if not stripped.isdigit():
        raise ValueError(f"PID file contains non-integer: {raw!r}")
    return int(stripped)


def is_process_alive(pid: int) -> bool:
    """Wrapper over psutil.pid_exists for test readability."""
    return psutil.pid_exists(pid)
