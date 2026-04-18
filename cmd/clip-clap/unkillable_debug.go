//go:build debug

// cmd/clip-clap — debug-build build-tag seam. When the binary is built
// with `go build -tags debug`, this file is included and sets the
// `unkillableDebugEnabled` package var to true, causing main.go's
// run() to register the `--unkillable-debug` flag. Release builds
// (default, no -tags debug) instead include unkillable_release.go
// which keeps the var false — passing `--unkillable-debug` to a
// release binary exits with `flag provided but not defined` per
// security-plan §Input Validation.
//
// Double gate on top of this build tag: even when parseable, the
// flag only activates when `CLIP_CLAP_TEST_UNKILLABLE=1` is set in
// the environment (see main.go's unkillableHookActive logic).
package main

// unkillableDebugEnabled is true only in debug builds. main.go reads
// this to decide whether to register the --unkillable-debug flag.
var unkillableDebugEnabled = true
