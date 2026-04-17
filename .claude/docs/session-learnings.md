# Session Learnings

_This file is curated by `/wrap-session`. Learnings captured here are too detailed or specific for CLAUDE.md but worth preserving as reference material for future sessions._

_Entries are added in reverse chronological order (newest first). Each entry has an ISO date, short title, and body._

_This file is entirely wrap-session's territory. `/setup-project` creates it if missing but NEVER regenerates it. Manual edits are preserved across all Andromeda skill runs._

---

## 2026-04-17 — Go pseudo-versions from Andromeda plans may not exist on proxy.golang.org

When an Andromeda gamma plan specifies a Go module at a pseudo-version like
`v0.0.0-YYYYMMDDHHMMSS-<12charSHA>`, the SHA suffix may be synthetic or stale
and not exist on the Go proxy. Architecture-table entries with version strings
that were never tagged upstream (e.g., `v0.0.4` for a module that only has
`master` commits) fall in the same trap.

**Pragmatic workflow before writing `go.mod` from a plan:**

1. For **tagged modules**, query: `go list -m -versions <module>` and confirm
   the plan's version is in the list.
2. For **pseudo-versioned modules**, query:
   `curl -sS https://proxy.golang.org/<module>/@latest`
   to get the actual latest commit SHA, and compare against the plan's SHA.
3. Resolve discrepancies by using the real latest; record the actual version
   in `docs/dependencies.md` with a review-date audit note per security-plan
   §Dependency Security.

**Examples from clip-clap Phase 0 (2026-04-17):**
- Architecture and security-plan listed `go-toast/toast v0.0.0-...01e3ca3626d8`
  — synthetic SHA, proxy returned `unknown revision`. Real latest was
  `01e6764cf0a4` from 2019-02-11.
- Architecture listed `kbinani/screenshot v0.0.4` — no such tag; actual
  latest on `master` is
  `v0.0.0-20250624051815-089614a94018`.

**Also note:** `go mod tidy` removes a `require` if no code imports it. For
Phase 0 (empty package stubs, no imports), use a `//go:build tools` file
(`tools/tools.go`) with blank `_` imports to keep the deps pinned in
`go.mod`. The `tools` build tag excludes the file from regular builds.

See: `go.mod`, `docs/dependencies.md`, `tools/tools.go`

---

## Entry format

Each entry follows this structure:

```
## {ISO-date} — {short title}
{1-3 paragraphs describing what was learned, why it matters, and where it applies. Reference specific files or documented decisions when relevant.}

See: `.claude/docs/...` or `internal/...` (cross-references)
```

## Tier classification

This file is **Tier 3 — on-demand**. Claude reads it when explicitly needed (debugging, planning, reviewing patterns), not at session start.

Other tiers:
- **Tier 1** (always loaded) — universal safety rules in `CLAUDE.md` `USER:session-learnings` section (critical, short)
- **Tier 2** (path-triggered) — directives in `.claude/rules/*.md` `## Session Additions` sections (loaded when matching files touched)
- **Tier 3** (on-demand) — this file (detailed reference, lazy-read)

## Promotion

When this file grows beyond ~200 lines, `/wrap-session` suggests promoting some entries to topic-specific files. Promotion is a user action, not automatic.

## Demotion from CLAUDE.md

If `CLAUDE.md` `USER:session-learnings` section gets too large (≥ 180 lines total CLAUDE.md), `/wrap-session` suggests promoting old Tier 1 entries down to this file (Tier 3) to keep CLAUDE.md within size budget.
