# Development Workflow

_Extracted from architecture.md and project conventions by `/setup-project`. Reference for how to actually work on this project day-to-day._

## Git workflow
- **Branch naming:** `feature/{description}`, `fix/{description}`, `chore/{description}`, `refactor/{area}`
- **Commit format:** conventional commits (`feat:`, `fix:`, `chore:`, `refactor:`, `docs:`, `test:`, `perf:`, `build:`, `ci:`)
- **Main branch:** `main`
- **Never force push** to `main`
- **PRs:** standard summary + test plan; use `gh pr create` not the web UI
- **Use `gh` CLI** for ALL GitHub operations (PRs, issues, CI status) per global preference

## Andromeda workflow

This project uses the Andromeda planning system for architecture and implementation planning.

**Initial setup (already done):**
1. `/init-project` — scaffold Claude Code infrastructure ✓
2. `/andromeda-alpha` — architecture planning ✓ (`.andromeda/architecture.md` exists)
3. `/setup-project` — populated this CLAUDE.md and `.claude/` from architecture ✓

**Per phase (main development loop, not yet started):**
1. `/andromeda-beta` — create masterplan from architecture
2. `/andromeda-omega` — design system + enrich masterplan with `### Design` per UI phase (this project has UI: systray, overlay, tray menu, toast)
3. `/andromeda-epsilon` — security plan + enrich masterplan with `### Security` per phase (this project handles clipboard, filesystem, loopback HTTP)
4. `/andromeda-gamma` — create implementation plan for next pending phase
5. `/implement` — execute the plan (writes code, tests, commits)
6. Mark phase completed in `project.yaml`
7. Repeat for next phase

**Why omega and epsilon matter for this project:** Without them, gamma extracts "None" for both Design and Security in Phase 0. The implementation plan then has zero design guidance (generic Windows UI defaults) and zero security guidance (insecure clipboard/filesystem patterns). Both are recommended before the first `/andromeda-gamma`.

## Daily session lifecycle

**At the start of a session:**
- `/new-session` — reads session-handoff, runs CLAUDE.md ecosystem health check, surfaces Andromeda status, proposes next action

**During work:**
- Follow the current Andromeda phase's `plan.md`
- Edit code, run `go test ./... -race`, iterate
- For UI changes, run the integration suite via `pwsh ./scripts/agent-run.ps1 build && pytest tests/integration/`
- When Claude makes a mistake, correct it in conversation — `/wrap-session` will capture the correction as a learning

**At the end of a session (or mid-session after a milestone):**
- `/wrap-session` — commits work, updates `.claude/session-handoff.md`, analyzes the session for learnings and curates them into the right tier (CLAUDE.md warnings, `.claude/rules/*.md`, or `.claude/docs/session-learnings.md`)

**Don't skip session boundaries.** `/new-session` + `/wrap-session` are the mechanism that keeps CLAUDE.md and project knowledge fresh over time.

## Code review

- **Locally:** `code-reviewer` agent is installed at `.claude/agents/code-reviewer.md` (Go-tailored) — invoke via "review this", "check this", "looks good?", or after implementing features
- **PRs:** use `@claude` tag on GitHub if Claude Code GitHub integration is configured
- **Architecture review:** for significant changes, read `.andromeda/architecture.md` Established Decisions to check alignment

## Release process

1. Verify CI is green on `main` for the current commit (`gh run list --branch main --limit 5`)
2. Update version in `goversioninfo.json` if needed
3. Run `govulncheck ./...` — patch any reported CVEs first
4. `git tag v1.x.y && git push --tags`
5. GitHub Actions runs `build-release` (compiles with `goversioninfo` + `go build`, optionally signs if `AZURE_SIGN_THUMBPRINT` secret present)
6. `publish-release` job uploads `.exe` to GitHub Releases and updates the Scoop manifest checksum
7. Verify the release page on GitHub; confirm the Scoop bucket commit went through
8. Smoke test: `scoop update clip-clap && clip-clap.exe` — confirm the published binary launches and registers the hotkey

## Troubleshooting workflow

- **Tests failing?** Check `.claude/rules/testing.md` for path-scoped conventions
- **Hooks not running?** Check `.claude/settings.json` and verify `gofmt` is on `PATH`
- **Integration test won't reach `ready=true`?** Check that `agent-run.ps1 kill` ran cleanly between tests; the named mutex blocks the new process if the prior one is still alive
- **CLAUDE.md looks stale?** Run `/new-session` to see the health check; possibly re-run `/setup-project`
- **`pytest` fails in fixture init?** UIA may be unavailable on the runner — check Anthropic GitHub Actions runner image release notes

## Philosophy

This project uses a **reference-based** CLAUDE.md architecture — a thin CLAUDE.md index with deeper content in `.claude/docs/` (on-demand) and `.claude/rules/` (path-scoped). See `section-markers.md` in any of the 4 Claude Code workflow skills for the ownership convention between generated and user-curated sections.
