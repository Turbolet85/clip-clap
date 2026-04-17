module github.com/Turbolet85/clip-clap

// Pinned to Go 1.23 (bumped from 1.22 per security-plan §Dependency Security
// to clear CVE-2025-47913 (CVSS 7.5), CVE-2025-47914, CVE-2025-58181 before
// 1.22 EOL. Architecture still says 1.22 — update pending).
go 1.23

require (
	github.com/go-toast/toast v0.0.0-20190211030409-01e6764cf0a4
	github.com/josephspurrier/goversioninfo v1.5.0
	github.com/kbinani/screenshot v0.0.0-20250624051815-089614a94018
	github.com/oklog/ulid/v2 v2.1.0
	github.com/pelletier/go-toml/v2 v2.2.2
)

require (
	github.com/akavel/rsrc v0.10.2 // indirect
	github.com/gen2brain/shm v0.1.0 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/jezek/xgb v1.1.1 // indirect
	github.com/lxn/win v0.0.0-20210218163916-a377121e959e // indirect
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d // indirect
	golang.org/x/sys v0.24.0 // indirect
)
