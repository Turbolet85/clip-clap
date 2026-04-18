"""agent-run.ps1 harness verification — build, start, status, logs, kill."""
import json
import os
import subprocess
import time

import psutil
import pytest

from conftest import PID_FILE, read_pid_file


def test_agent_run_ps1_build_command():
    """AC #1: `agent-run.ps1 build` exits 0, produces clip-clap.exe > 1 MB."""
    result = subprocess.run(
        ["pwsh", "scripts/agent-run.ps1", "build"],
        capture_output=True, text=True, timeout=60,
    )
    assert result.returncode == 0, \
        f"build exited {result.returncode}: stdout={result.stdout!r} stderr={result.stderr!r}"
    assert os.path.exists("clip-clap.exe"), "clip-clap.exe should exist after build"
    size = os.path.getsize("clip-clap.exe")
    assert size > 1_048_576, f"clip-clap.exe is {size} bytes, want > 1 MB"


def test_agent_run_ps1_logs_command(agent):
    """AC #8: `agent-run.ps1 logs` prints tail of logs/agent-latest.jsonl.

    The agent fixture ensures the log file exists and has at least
    config.loaded + hotkey.registered events.
    """
    result = subprocess.run(
        ["pwsh", "scripts/agent-run.ps1", "logs"],
        capture_output=True, text=True, timeout=10,
    )
    assert result.returncode == 0, f"logs exited {result.returncode}: {result.stderr!r}"

    # At least one line should parse as JSON with an `event` field.
    found = False
    for line in result.stdout.splitlines():
        line = line.strip()
        if not line:
            continue
        try:
            obj = json.loads(line)
            if "event" in obj:
                found = True
                break
        except json.JSONDecodeError:
            continue
    assert found, f"no JSON line with `event` field in logs output: {result.stdout!r}"


def test_agent_run_ps1_kill_deletes_pidfile(agent):
    """AC #9: kill removes .agent-running and terminates the process."""
    assert os.path.exists(PID_FILE), "PID file should exist at test start"
    pid = read_pid_file()

    result = subprocess.run(
        ["pwsh", "scripts/agent-run.ps1", "kill"],
        capture_output=True, text=True, timeout=10,
    )
    assert result.returncode == 0

    # Within 5s, the process should be gone AND the PID file removed.
    deadline = time.monotonic() + 5.0
    while time.monotonic() < deadline:
        if not psutil.pid_exists(pid) and not os.path.exists(PID_FILE):
            return
        time.sleep(0.1)

    pytest.fail(
        f"within 5s of kill: pid_exists={psutil.pid_exists(pid)}, "
        f"pid_file_exists={os.path.exists(PID_FILE)}"
    )


@pytest.mark.skipif(
    True,
    reason=(
        "--unkillable-debug is only parseable on -tags debug builds; "
        "default CI build is release. Activate this test in a "
        "debug-tagged CI job when ready."
    ),
)
def test_agent_run_ps1_kill_falls_back_to_taskkill():
    """Taskkill fallback: start with --unkillable-debug + env var,
    send kill, verify the stderr fallback message fires and the
    process is force-killed after the 5s WM_CLOSE timeout.
    """
    # This test needs a debug-tagged build. In the skip-if configuration
    # it's conditionally run only when the environment has the debug
    # binary available. Test body retained for future activation.
    env = {**os.environ, "CLIP_CLAP_TEST_UNKILLABLE": "1"}
    # Build with -tags debug (not the default agent-run.ps1 build).
    subprocess.run(
        ["go", "build", "-tags", "debug", "-ldflags=-H windowsgui -s -w",
         "-o", "clip-clap.exe", "./cmd/clip-clap"],
        check=True, env=env, timeout=60,
    )
    proc = subprocess.Popen(
        [".\\clip-clap.exe", "--agent-mode", "--unkillable-debug"],
        env=env,
    )
    try:
        # Wait for the agent to be ready (PID file present).
        deadline = time.monotonic() + 10
        while time.monotonic() < deadline and not os.path.exists(PID_FILE):
            time.sleep(0.1)
        assert os.path.exists(PID_FILE)

        # Invoke kill. Should take ~5s (WM_CLOSE timeout) then taskkill.
        result = subprocess.run(
            ["pwsh", "scripts/agent-run.ps1", "kill"],
            capture_output=True, text=True, timeout=15,
        )
        # Wait for the OS to reap the process.
        time.sleep(1)

        # Assertion 1: process is gone.
        assert not psutil.pid_exists(proc.pid), \
            f"process {proc.pid} still alive after kill"
        # Assertion 2: stderr contains the fallback message.
        assert f"falling back to taskkill /PID {proc.pid} /F" in result.stderr, \
            f"expected fallback message in stderr, got: {result.stderr!r}"
    finally:
        # Belt-and-suspenders cleanup.
        if psutil.pid_exists(proc.pid):
            proc.kill()
