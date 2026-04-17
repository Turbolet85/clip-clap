# Session Learnings

_This file is curated by `/wrap-session`. Learnings captured here are too detailed or specific for CLAUDE.md but worth preserving as reference material for future sessions._

_Entries are added in reverse chronological order (newest first). Each entry has an ISO date, short title, and body._

_This file is entirely wrap-session's territory. `/setup-project` creates it if missing but NEVER regenerates it. Manual edits are preserved across all Andromeda skill runs._

---

## 2026-04-17 — Explore sub-agents reliably prepend conversational preamble; strip-and-save-raw is the pragmatic response

When spawning Explore-type sub-agents via the Agent tool with strict `NO_PREAMBLE` validation (e.g., "first character of your response must be `#`", "no 'Based on my read...' or 'Perfect! I now have...'"), sub-agents still produce 2–3 lines of conversational preamble before the required template start. Observed 6+ times across a single `/andromeda-gamma` run (Phase 0 task extraction, Phase 1 research, Phase 2 draft, Phase 3 meta-prompt, Phase 3a refinement — every sub-agent invocation).

The pragmatic orchestrator response — authorized explicitly by the user on the first occurrence and carried forward through the run:

1. **Save the full raw sub-agent output** to `{run_dir}/.raw-<filename>.md` for audit. This preserves Pass 1–5 analysis, preamble, and any rationale that might be useful later but shouldn't land in the final artifact.
2. **Manually strip the preamble** when writing the cleaned content to the final artifact (e.g., `task.md`, `research.md`, `plan-draft.md`). The post-preamble template is usually well-filled; the sub-agent's CONTENT quality is good, only its output DRESSING fails validation.
3. **Report the residual in the orchestrator's dashboard** ("preamble stripped; raw saved to .raw-*.md") so the user sees what happened without having to read the raw file.
4. **Do NOT halt the pipeline** on NO_PREAMBLE failure alone — the halt-after-2-failures contract is appropriate for content-quality failures (missing sections, empty AC lists) but overly strict for cosmetic preamble.

This pattern applies to every Andromeda orchestrator skill that spawns sub-agents (alpha, beta, gamma, omega, epsilon, sigma). The validation framework already supports it — validation checks other than NO_PREAMBLE usually pass, and the preamble is trivially strippable.

See: `.andromeda/runs/2026-04-17T14-47-51-gamma-phase-1/.raw-*.md` (seven concrete examples of the pattern and its resolution in a single run).

---

## 2026-04-17 — Gamma plans can contain internal contradictions between test specs and AC contracts

`/andromeda-gamma` extracts test specifications (Phase 2a QA sub-agent) and step specifications (Phase 2 first-draft sub-agent) independently from different source sections — `task-tests.md` for tests, `task.md` for ACs. The two extractions are not cross-validated, so the resulting `plan-draft.md` can contain a test whose premise contradicts the AC contract the test is nominally meant to cover.

**Concrete example from clip-clap Phase 1 (2026-04-17):** task-tests.md specified `TestLoad_AutoCreateOnMissingFile` with setup "set temp config path via `CLIP_CLAP_CONFIG` env var to a non-existent file... assert the file was created with default values". But task.md + AC #7 require CLIP_CLAP_CONFIG + missing-file to RETURN AN ERROR (only the DEFAULT `%APPDATA%\clip-clap\config.toml` path auto-creates on first run). The test's setup directly contradicted the AC contract.

**Resolution during /implement:** trust the AC contract (architecture-level authority); adjust the test's SETUP to reach the same INTENT through the correct path. Here: redirect `AppData` and `XDG_CONFIG_HOME` env vars to a TempDir so `os.UserConfigDir()` returns the temp location, then call `Load()` without setting `CLIP_CLAP_CONFIG` — exercises the default-path auto-create that AC #1 intends. The CLIP_CLAP_CONFIG-missing case is tested separately by `TestLoad_CustomConfigMissingFile` per AC #7 (now two distinct tests for two distinct contracts).

**Lesson for future `/implement` runs:** when a test spec in a plan seems to conflict with an AC it's meant to cover, **trust the AC** (architecture-level) and adjust the test setup to reach the same INTENT through the correct path. Document the adjustment in the implement-progress.md audit trail (§Adaptations / deltas from plan). If the contradiction is systemic (multiple tests misaligned), halt and ask the user before proceeding.

See: `.andromeda/runs/2026-04-17T14-47-51-gamma-phase-1/implement-progress.md` §Adaptations (finding #1).

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
