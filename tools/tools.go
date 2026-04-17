//go:build tools
// +build tools

// Package tools pins build-time dependencies so `go mod tidy` keeps them
// in go.mod even when no production code has imported them yet (Phase 0
// foundation skeleton has empty internal packages).
//
// This file is excluded from regular builds by the `tools` build tag.
package tools

import (
	_ "github.com/go-toast/toast"
	_ "github.com/josephspurrier/goversioninfo"
	_ "github.com/kbinani/screenshot"
	_ "github.com/oklog/ulid/v2"
	_ "github.com/pelletier/go-toml/v2"
)
