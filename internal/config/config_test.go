package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// redirectUserHomeDir points os.UserHomeDir at the given dir for the
// duration of the test, without mutating the real %USERPROFILE% / $HOME.
// Using t.Setenv gives automatic restore on test cleanup.
//
// v1.0.8 note: default config path was moved from os.UserConfigDir
// (AppData on Windows) to os.UserHomeDir + Pictures\clip-clap, so this
// helper now redirects USERPROFILE / HOME instead of AppData /
// XDG_CONFIG_HOME.
func redirectUserHomeDir(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("USERPROFILE", dir) // Windows
	t.Setenv("HOME", dir)        // Linux / macOS
	// Clear env vars that would shadow the default-path auto-create flow.
	t.Setenv("CLIP_CLAP_CONFIG", "")
}

// TestLoad_AutoCreateOnMissingFile — exercise the default-path auto-create
// branch: when CLIP_CLAP_CONFIG is unset and the default config.toml under
// %USERPROFILE%\Pictures\clip-clap\ does not exist, Load writes a defaults
// file and decodes it successfully. Covers AC #1. (CLIP_CLAP_CONFIG +
// missing file is covered separately by TestLoad_CustomConfigMissingFile
// per AC #7 — those are two different contracts per architecture
// §Config Management.)
func TestLoad_AutoCreateOnMissingFile(t *testing.T) {
	tmp := t.TempDir()
	redirectUserHomeDir(t, tmp)

	cfg, cfgPath, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	expectedPath := filepath.Join(tmp, "Pictures", "clip-clap", "config.toml")
	if cfgPath != expectedPath {
		t.Errorf("cfgPath mismatch\n  want: %s\n  got:  %s", expectedPath, cfgPath)
	}
	if _, err := os.Stat(cfgPath); err != nil {
		t.Fatalf("config file not created at %s: %v", cfgPath, err)
	}
	// All four documented keys populated with their defaults.
	if cfg.Hotkey != defaultHotkey {
		t.Errorf("Hotkey\n  want: %q\n  got:  %q", defaultHotkey, cfg.Hotkey)
	}
	if cfg.AutoQuotePaths != defaultAutoQuotePaths {
		t.Errorf("AutoQuotePaths\n  want: %t\n  got:  %t", defaultAutoQuotePaths, cfg.AutoQuotePaths)
	}
	if cfg.LogLevel != defaultLogLevel {
		t.Errorf("LogLevel\n  want: %q\n  got:  %q", defaultLogLevel, cfg.LogLevel)
	}
	if !strings.Contains(cfg.SaveFolder, "clip-clap") {
		t.Errorf("SaveFolder should include 'clip-clap' component; got: %q", cfg.SaveFolder)
	}
}

// TestLoad_StrictModeRejectsUnknownKey — unknown keys in config.toml cause
// Load to return an error. Covers AC #8 and security-plan §Input Validation.
func TestLoad_StrictModeRejectsUnknownKey(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.toml")
	if err := os.WriteFile(cfgPath, []byte(`save_folder = "D:\\x"`+"\n"+`hotkey = "Ctrl+Shift+S"`+"\n"+`auto_quote_paths = true`+"\n"+`log_level = "INFO"`+"\n"+`bogus_key = "x"`+"\n"), 0o644); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	t.Setenv("CLIP_CLAP_CONFIG", cfgPath)

	_, _, err := Load()
	if err == nil {
		t.Fatal("expected error for unknown key; got nil")
	}
	if !strings.Contains(err.Error(), "bogus_key") {
		t.Errorf("error should mention 'bogus_key' (the offending key); got: %v", err)
	}
}

// TestLoad_EnvVarPrecedence — CLIP_CLAP_SAVE_DIR overrides the file's
// save_folder key. Covers AC #6.
func TestLoad_EnvVarPrecedence(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.toml")
	if err := os.WriteFile(cfgPath, []byte(`save_folder = "D:\\FromFile\\"`+"\n"+`hotkey = "Ctrl+Shift+S"`+"\n"+`auto_quote_paths = true`+"\n"+`log_level = "INFO"`+"\n"), 0o644); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	t.Setenv("CLIP_CLAP_CONFIG", cfgPath)
	t.Setenv("CLIP_CLAP_SAVE_DIR", `C:\Captures\`)

	cfg, _, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.SaveFolder != `C:\Captures\` {
		t.Errorf("env var should override file\n  want: %q\n  got:  %q", `C:\Captures\`, cfg.SaveFolder)
	}
}

// TestLoad_EnvVarDebugOverride — table-driven validation of all documented
// CLIP_CLAP_DEBUG values per security-plan §Input Validation. Covers AC #5.
func TestLoad_EnvVarDebugOverride(t *testing.T) {
	cases := []struct {
		name      string
		envValue  string
		wantLevel string
		wantErr   bool
	}{
		{"value-1 forces DEBUG", "1", debugLogLevel, false},
		{"value-true forces DEBUG", "true", debugLogLevel, false},
		{"value-0 leaves file level intact", "0", defaultLogLevel, false},
		{"value-false leaves file level intact", "false", defaultLogLevel, false},
		{"empty string leaves file level intact", "", defaultLogLevel, false},
		{"invalid string returns error", "invalid", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			cfgPath := filepath.Join(tmp, "config.toml")
			if err := os.WriteFile(cfgPath, []byte(`save_folder = "D:\\x\\"`+"\n"+`hotkey = "Ctrl+Shift+S"`+"\n"+`auto_quote_paths = true`+"\n"+`log_level = "INFO"`+"\n"), 0o644); err != nil {
				t.Fatalf("seed config: %v", err)
			}
			t.Setenv("CLIP_CLAP_CONFIG", cfgPath)
			t.Setenv("CLIP_CLAP_DEBUG", tc.envValue)

			cfg, _, err := Load()
			if tc.wantErr {
				if err == nil {
					t.Error("expected error for invalid CLIP_CLAP_DEBUG; got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Load returned unexpected error: %v", err)
			}
			if cfg.LogLevel != tc.wantLevel {
				t.Errorf("LogLevel\n  want: %q\n  got:  %q", tc.wantLevel, cfg.LogLevel)
			}
		})
	}
}

// TestLoad_CustomConfigMissingFile — CLIP_CLAP_CONFIG pointing to a non-
// existent absolute path returns an error per architecture §Config Management.
// Covers AC #7.
func TestLoad_CustomConfigMissingFile(t *testing.T) {
	tmp := t.TempDir()
	missing := filepath.Join(tmp, "does-not-exist.toml")
	t.Setenv("CLIP_CLAP_CONFIG", missing)

	_, _, err := Load()
	if err == nil {
		t.Fatal("expected error for missing CLIP_CLAP_CONFIG file; got nil")
	}
	if !strings.Contains(err.Error(), "missing file") {
		t.Errorf("error should mention 'missing file'; got: %v", err)
	}
}
