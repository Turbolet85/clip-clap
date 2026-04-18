// Package config owns the strict-mode TOML config parser
// (pelletier/go-toml/v2 with DisallowUnknownFields) backed by
// %APPDATA%\clip-clap\config.toml plus the CLIP_CLAP_* environment-variable
// override layer.
//
// Phase 1 implements the real loader. Load() returns a resolved *Config,
// the resolved config path (for audit logging as filepath.Base only), and
// any error. On first run, the config file is auto-created with defaults;
// on subsequent runs, the existing file is decoded in strict mode
// (unknown keys rejected per security-plan §Input Validation).
package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Default values per architecture §First-Run Behavior. Constants (not a config
// instance) so defaults never accidentally mutate at runtime.
const (
	defaultHotkey         = "Ctrl+Shift+S"
	defaultAutoQuotePaths = true
	defaultLogLevel       = "INFO"
	debugLogLevel         = "DEBUG"
)

// Config is the decoded content of config.toml plus env-var overrides.
type Config struct {
	SaveFolder     string `toml:"save_folder"`
	Hotkey         string `toml:"hotkey"`
	AutoQuotePaths bool   `toml:"auto_quote_paths"`
	LogLevel       string `toml:"log_level"`
}

// Load resolves the config file path (env var or default), auto-creates a
// defaults file on first run at the default path, decodes in strict mode, and
// applies validated env-var overrides. Returns (*Config, cfgPath, error).
//
// Env var precedence (highest first):
//  1. CLIP_CLAP_CONFIG — if set, used as the config file path (must be absolute;
//     file must exist or Load returns error per architecture §Config Management).
//  2. CLIP_CLAP_SAVE_DIR — overrides cfg.SaveFolder (must be absolute).
//  3. CLIP_CLAP_DEBUG — bool-ish string ("1"/"true"/"0"/"false"/empty); "1" or
//     "true" force LogLevel to DEBUG. Any other value is rejected with an error
//     per security-plan §Input Validation.
//
// CLIP_CLAP_LOG_PATH is read separately by the caller in cmd/clip-clap/main.go
// (it affects the logger, not config); this function validates its format only
// if the caller chooses to pass it through — not all config paths need it.
func Load() (*Config, string, error) {
	cfgPath, fromEnv, err := resolveConfigPath()
	if err != nil {
		return nil, "", err
	}
	if err := ensureConfigFile(cfgPath, fromEnv); err != nil {
		return nil, "", err
	}
	cfg, err := decodeStrict(cfgPath)
	if err != nil {
		return nil, "", err
	}
	if err := applyEnvOverrides(cfg); err != nil {
		return nil, "", err
	}
	return cfg, cfgPath, nil
}

// resolveConfigPath returns (path, fromEnv, err). If CLIP_CLAP_CONFIG is set,
// its value is validated and returned (fromEnv=true). Otherwise the default
// %USERPROFILE%\Pictures\clip-clap\config.toml is returned.
//
// v1.0.8 co-locates config.toml with screenshots (same directory as the
// default save_folder) so users see it next to their captures instead of
// hidden under %APPDATA%. Even if the user later points save_folder
// elsewhere in config.toml, the config file itself stays in
// Pictures\clip-clap\ — otherwise we'd have a chicken-and-egg problem
// (can't read save_folder from config.toml if config.toml location
// depends on save_folder).
func resolveConfigPath() (string, bool, error) {
	if v := os.Getenv("CLIP_CLAP_CONFIG"); v != "" {
		if !filepath.IsAbs(v) {
			return "", false, fmt.Errorf("CLIP_CLAP_CONFIG must be an absolute path (got relative: %q)", filepath.Base(v))
		}
		if strings.HasPrefix(v, `\\`) {
			return "", false, fmt.Errorf("CLIP_CLAP_CONFIG must not be a UNC path (got %q)", filepath.Base(v))
		}
		return v, true, nil
	}
	dir, err := DefaultDataDir()
	if err != nil {
		return "", false, err
	}
	return filepath.Join(dir, "config.toml"), false, nil
}

// DefaultDataDir returns %USERPROFILE%\Pictures\clip-clap — the canonical
// location for clip-clap's config.toml, logs/, and default save folder.
// Exported so cmd/clip-clap/main.go can derive the default log path from
// the same base as the config path (v1.0.8 colocation).
func DefaultDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home dir: %w", err)
	}
	return filepath.Join(home, "Pictures", "clip-clap"), nil
}

// ensureConfigFile creates the default path with hardcoded defaults if it does
// not exist AND CLIP_CLAP_CONFIG was not used. If CLIP_CLAP_CONFIG was used and
// the file is missing, return an error (do NOT auto-create at a user-supplied
// path — that would surprise the user per architecture §Config Management).
func ensureConfigFile(cfgPath string, fromEnv bool) error {
	if _, err := os.Stat(cfgPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat config file %s: %w", filepath.Base(cfgPath), err)
	}
	if fromEnv {
		return fmt.Errorf("CLIP_CLAP_CONFIG points to missing file %s", filepath.Base(cfgPath))
	}
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		return fmt.Errorf("create config dir %s: %w", filepath.Base(filepath.Dir(cfgPath)), err)
	}
	contents, err := renderDefaultTOML()
	if err != nil {
		return fmt.Errorf("render default config: %w", err)
	}
	if err := os.WriteFile(cfgPath, contents, 0o644); err != nil {
		return fmt.Errorf("write default config %s: %w", filepath.Base(cfgPath), err)
	}
	return nil
}

// renderDefaultTOML produces the first-run defaults file. save_folder is
// resolved to an actual path (e.g., C:\Users\alice\Pictures\clip-clap\) rather
// than the unexpanded `%USERPROFILE%` literal — keeps the config file readable
// to the user who edits it later and avoids runtime env-var expansion in Load.
func renderDefaultTOML() ([]byte, error) {
	dir, err := DefaultDataDir()
	if err != nil {
		return nil, err
	}
	saveFolder := dir + string(filepath.Separator)
	var b bytes.Buffer
	fmt.Fprintf(&b, "# clip-clap configuration — edit by hand, restart to apply.\n")
	fmt.Fprintf(&b, "# Strict mode rejects unknown keys (typos become startup errors).\n")
	fmt.Fprintf(&b, "#\n")
	fmt.Fprintf(&b, "# Hotkey format — CASE-SENSITIVE, capitalize only the first letter:\n")
	fmt.Fprintf(&b, "#   Modifiers: Ctrl, Shift, Alt, Win  (rejected: ctrl, CTRL, ShIfT, etc.)\n")
	fmt.Fprintf(&b, "#   Keys: A-Z, 0-9, F1-F12, Space, PageUp, PageDown, Home, End, Insert, Delete\n")
	fmt.Fprintf(&b, "#   Combine with +, no spaces: Ctrl+Shift+S | Alt+F4 | Win+V | Ctrl+Shift+PageUp\n")
	fmt.Fprintf(&b, "# If parsing fails, clip-clap logs hotkey.error at startup and runs\n")
	fmt.Fprintf(&b, "# without an active hotkey (tray Expose still works).\n\n")
	fmt.Fprintf(&b, "save_folder = %q\n", saveFolder)
	fmt.Fprintf(&b, "hotkey = %q\n", defaultHotkey)
	fmt.Fprintf(&b, "auto_quote_paths = %t\n", defaultAutoQuotePaths)
	fmt.Fprintf(&b, "log_level = %q\n", defaultLogLevel)
	return b.Bytes(), nil
}

// decodeStrict reads cfgPath and decodes it with DisallowUnknownFields. Unknown
// keys surface as *toml.StrictMissingError; the caller (cmd/clip-clap/main.go
// Step 11) presents this to the user via stderr. Error messages include the
// specific offending key(s) via StrictMissingError.String() — the default
// Error() method is terse and does not name the key.
func decodeStrict(cfgPath string) (*Config, error) {
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", filepath.Base(cfgPath), err)
	}
	cfg := &Config{}
	dec := toml.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(cfg); err != nil {
		var strict *toml.StrictMissingError
		if errors.As(err, &strict) {
			// StrictMissingError.String() provides the full report with the
			// offending key names and line numbers; the default Error() method
			// is terse ("strict mode: fields are missing...") without naming
			// the key, which is actively unhelpful for users debugging typos.
			return nil, fmt.Errorf("decode config %s (strict mode rejected unknown keys): %s", filepath.Base(cfgPath), strict.String())
		}
		return nil, fmt.Errorf("decode config %s: %w", filepath.Base(cfgPath), err)
	}
	return cfg, nil
}

// applyEnvOverrides mutates cfg in place from validated env vars.
func applyEnvOverrides(cfg *Config) error {
	if v := os.Getenv("CLIP_CLAP_SAVE_DIR"); v != "" {
		if !filepath.IsAbs(v) {
			return fmt.Errorf("CLIP_CLAP_SAVE_DIR must be an absolute path (got relative value)")
		}
		cfg.SaveFolder = v
	}
	if v := os.Getenv("CLIP_CLAP_DEBUG"); v != "" {
		switch strings.ToLower(v) {
		case "1", "true":
			cfg.LogLevel = debugLogLevel
		case "0", "false":
			// explicit off — leave cfg.LogLevel as decoded from file
		default:
			return fmt.Errorf("CLIP_CLAP_DEBUG must be 1/true/0/false (got invalid value)")
		}
	}
	return nil
}
