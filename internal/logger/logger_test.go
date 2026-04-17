package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// rfc3339NanoRegex asserts the time field written by our custom ReplaceAttr —
// RFC 3339 with 9-digit nanosecond precision and a timezone indicator (Z or
// ±HH:MM). The stdlib's default slog time format is second-precision only, so
// this regex is the canonical check that our override fires.
var rfc3339NanoRegex = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{9}([Zz]|[+-]\d{2}:\d{2})$`)

// withOriginalDefault captures the current slog default at test entry and
// restores it via t.Cleanup. Logger.Initialize calls slog.SetDefault globally;
// without this restore, earlier tests can pollute later tests by leaking their
// handler/level settings.
func withOriginalDefault(t *testing.T) {
	t.Helper()
	original := slog.Default()
	t.Cleanup(func() {
		slog.SetDefault(original)
	})
}

func TestInitialize_EmitsRFC3339Nano(t *testing.T) {
	withOriginalDefault(t)

	var buf bytes.Buffer
	if err := InitializeWithWriter(slog.LevelInfo, &buf); err != nil {
		t.Fatalf("InitializeWithWriter returned error: %v", err)
	}
	slog.Info("test record", "event", EventConfigLoaded)

	line := buf.String()
	if line == "" {
		t.Fatal("no log record written to buffer")
	}
	var record map[string]any
	if err := json.Unmarshal([]byte(line), &record); err != nil {
		t.Fatalf("log line is not valid JSON: %v\nline: %q", err, line)
	}
	timeVal, ok := record["time"].(string)
	if !ok {
		t.Fatalf("record has no 'time' string field; got: %v", record)
	}
	if !rfc3339NanoRegex.MatchString(timeVal) {
		t.Errorf("time field does not match RFC 3339 nanosecond format\nwant regex: %s\ngot:        %q", rfc3339NanoRegex.String(), timeVal)
	}
}

func TestInitialize_RespectsLevelParameter(t *testing.T) {
	t.Run("LevelDebug emits both Debug and Info", func(t *testing.T) {
		withOriginalDefault(t)
		var buf bytes.Buffer
		if err := InitializeWithWriter(slog.LevelDebug, &buf); err != nil {
			t.Fatalf("InitializeWithWriter returned error: %v", err)
		}
		slog.Debug("debug msg")
		slog.Info("info msg")
		out := buf.String()
		if !strings.Contains(out, `"msg":"debug msg"`) {
			t.Errorf("LevelDebug should emit Debug record; output: %q", out)
		}
		if !strings.Contains(out, `"msg":"info msg"`) {
			t.Errorf("LevelDebug should emit Info record; output: %q", out)
		}
	})

	t.Run("LevelInfo filters Debug but emits Info", func(t *testing.T) {
		withOriginalDefault(t)
		var buf bytes.Buffer
		if err := InitializeWithWriter(slog.LevelInfo, &buf); err != nil {
			t.Fatalf("InitializeWithWriter returned error: %v", err)
		}
		slog.Debug("debug msg")
		slog.Info("info msg")
		out := buf.String()
		if strings.Contains(out, `"msg":"debug msg"`) {
			t.Errorf("LevelInfo should filter Debug record; output: %q", out)
		}
		if !strings.Contains(out, `"msg":"info msg"`) {
			t.Errorf("LevelInfo should emit Info record; output: %q", out)
		}
	})
}

func TestInitialize_CreatesLogsDir(t *testing.T) {
	withOriginalDefault(t)

	tmp := t.TempDir()
	// Cleanup order is LIFO: register Close AFTER t.TempDir so our handle
	// close runs BEFORE TempDir's RemoveAll. Windows refuses to unlink a file
	// still held open by the process, so the opposite order leaks a FAIL.
	t.Cleanup(func() { _ = Close() })

	logPath := filepath.Join(tmp, "nested", "subdir", "agent-latest.jsonl")
	if err := Initialize(slog.LevelInfo, logPath); err != nil {
		t.Fatalf("Initialize returned error: %v", err)
	}
	parent := filepath.Dir(logPath)
	info, err := os.Stat(parent)
	if err != nil {
		t.Fatalf("parent directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("parent path exists but is not a directory: %s", parent)
	}
}
