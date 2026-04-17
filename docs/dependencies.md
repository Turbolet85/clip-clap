# Dependencies Audit Trail

Per `security-plan.md` §Dependency Security, every pinned version in `go.mod`
is recorded here with the date reviewed and the rationale. Refuse to bump any
entry without re-review.

## Direct dependencies

### github.com/pelletier/go-toml/v2 v2.2.2
- **Pinned:** 2026-04-17
- **Rationale:** Security plan §Dependency Security requires v2.2.2 (bumped
  from architecture's v2.2.0) to pick up the upstream vulnerability fix in
  v2.2.2. Newer versions (v2.2.3, v2.2.4, v2.3.0) exist but we pin exact to
  prevent range-based drift.
- **Next review:** before any v1.1 release.

### github.com/kbinani/screenshot v0.0.0-20250624051815-089614a94018
- **Pinned:** 2026-04-17
- **Rationale:** No tagged releases in upstream; pseudo-version pins the
  latest commit on `master` at review time. Commit `089614a94018` from
  2025-06-24. Architecture-compliance deviation: architecture table says
  `v0.0.4`, which does not exist on the proxy — replaced with real latest
  pseudo-version. Architecture should be updated.
- **go mod why:** used by `internal/capture/capture.go` (Phase 3) for
  `screenshot.CaptureRect` against a selected overlay region.
- **Next review:** before Phase 3 (first real use).

### github.com/oklog/ulid/v2 v2.1.0
- **Pinned:** 2026-04-17
- **Rationale:** Security plan §Win32 Resource Hygiene mandates monotonic
  entropy construction via `ulid.Monotonic(crypto/rand.Reader, 0)`. v2.1.0
  is the version matching that API. v2.1.1 exists but is not in the
  security plan's reviewed-versions list.
- **Next review:** before Phase 1 (first real use).

### github.com/go-toast/toast v0.0.0-20190211030409-01e6764cf0a4
- **Pinned:** 2026-04-17
- **Rationale:** Upstream `github.com/go-toast/toast` is frozen since 2019.
  Pseudo-version pins commit `01e6764cf0a4` from 2019-02-11 — the latest
  commit on `master`. NOT using the `stuartleeks/toast` fork (unmaintained
  since 2019, adds no security-reviewed commits per security plan §Dependency
  Security). Architecture-compliance deviation: architecture uses suffix
  `01e3ca3626d8`, which does not exist on the proxy — corrected to the real
  last-commit suffix. Architecture should be updated.
- **go mod why:** used by `internal/toast/toast.go` (Phase 3) for Windows
  toast notifications via `ToastNotificationManager`.
- **Reviewed by:** Turbolet85, 2026-04-17.
- **Next review:** before any v1.1 release, OR immediately if upstream ever
  publishes a new commit.

### github.com/josephspurrier/goversioninfo v1.5.0
- **Pinned:** 2026-04-17
- **Rationale:** Architecture §Resource Embedding. Consumed both as library
  (`tools.go` blank import to keep in go.mod) and as command-line tool
  (`goversioninfo.exe` installed to `$GOPATH/bin` via `go install
  github.com/josephspurrier/goversioninfo/cmd/goversioninfo@v1.5.0`).
  Invoked at build time by the `//go:generate` directive in
  `cmd/clip-clap/main.go` to produce `cmd/clip-clap/resource.syso` from
  `goversioninfo.json` + `assets/app.ico` + `assets/app.manifest`.
- **Next review:** before any v1.1 release.

## Update policy

- Versions are **exact pins** (no `>=`, `~`, or `latest`).
- `go.sum` is committed; CI runs `go mod verify` + asserts `go mod tidy`
  produces no diff.
- Quarterly dependency review: check `govulncheck` output, bump only if a
  CVE is flagged or a security-plan-mandated version change is approved.
- Go toolchain bumps (currently pinned at `go 1.23`): check before 1.23 EOL.
