// Package config owns the strict-mode TOML config parser
// (pelletier/go-toml/v2 with DisallowUnknownFields) backed by
// %APPDATA%\clip-clap\config.toml plus the CLIP_CLAP_* environment-variable
// override layer. Phase 1 implements the real loader; Phase 0 stubs the
// package so the directory exists and `go build ./...` succeeds.
package config

// Initialize is a placeholder; Phase 1 replaces it with the real config
// loader entry point.
func Initialize() error { return nil }
