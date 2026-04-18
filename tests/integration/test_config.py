"""Config file + environment variable override tests — Phase 4."""
import os
import subprocess
import tempfile

import pytest

from conftest import parse_log_events


def test_custom_config_missing_file_exits_nonzero():
    """CLIP_CLAP_CONFIG → nonexistent path → agent exits non-zero with config.error."""
    env = {**os.environ, "CLIP_CLAP_CONFIG": r"C:\nope\missing.toml"}
    result = subprocess.run(
        [".\\clip-clap.exe", "--agent-mode"],
        env=env, capture_output=True, text=True, timeout=5,
    )
    assert result.returncode != 0, \
        f"agent should exit non-zero with missing config, got {result.returncode}"
    # The log file path is determined by default; may or may not be populated
    # in time. Skip the log assertion if the log file wasn't created.
    events = parse_log_events()
    config_errors = [e for e in events if e.get("event") == "config.error"]
    # Accept either: config.error emitted, OR stderr contains config-load error
    # (the latter is what happens if logger init failed before config.error
    # could be written).
    if not config_errors:
        assert "config load failed" in result.stderr.lower() or \
               "missing.toml" in result.stderr.lower(), \
            f"expected config error signal, got stderr={result.stderr!r} events={events}"


def test_unknown_key_exits_nonzero():
    """Unknown TOML key in config → strict unmarshal rejects → exit non-zero."""
    with tempfile.NamedTemporaryFile(
        mode="w", suffix=".toml", delete=False, encoding="utf-8",
    ) as fp:
        fp.write('save_folder = "x"\nbogus = 1\n')
        temp_config = fp.name

    try:
        env = {**os.environ, "CLIP_CLAP_CONFIG": temp_config}
        result = subprocess.run(
            [".\\clip-clap.exe", "--agent-mode"],
            env=env, capture_output=True, text=True, timeout=5,
        )
        assert result.returncode != 0, \
            f"agent should reject unknown TOML key, got exit {result.returncode}"
    finally:
        os.remove(temp_config)


@pytest.mark.skip(
    reason=(
        "Phase 4: save-dir override happy-path requires a capture to "
        "land a PNG; skip until UIA drag is reliably exercised."
    )
)
def test_save_dir_env_override_lands_png(agent):
    """CLIP_CLAP_SAVE_DIR → capture lands in that folder, not default."""
    pass
