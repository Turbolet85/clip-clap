//go:build !debug

// cmd/clip-clap — release-build build-tag seam. Default build (no
// -tags debug) includes this file; `unkillableDebugEnabled = false`
// means main.go's run() NEVER registers the `--unkillable-debug`
// flag. Passing `--unkillable-debug` to a release binary therefore
// exits with `flag provided but not defined` (Go's flag package
// default behavior for unknown flags in ContinueOnError mode).
//
// This is the enforcement boundary for security-plan §Input
// Validation: release builds must reject the test-only flag.
package main

// unkillableDebugEnabled is false in release builds. This compile-time
// constant makes main.go's `if unkillableDebugEnabled { fs.Bool(...) }`
// branch unreachable, so the flag itself is never even added to the
// flag set.
var unkillableDebugEnabled = false
