---
kind: sprint
ticket: 1237
date: 2026-11-19
branch: sprint/taia-harnez-pack
---

# Two-Agent Lean Sprint: 1237 — Forge-Tracker Friction (Private Search, `find next`, Label Warnings)

## 1. Scout & Consult Audit
- **Scout (`search-1237`, `claude:haiku:med`, 42s)**: Located `forgeTrackerHint` / `warnIfForgeAuthErr` (`cmd/harnez/issues_forge.go`), `PlanCreate` / `formatInboxIssue` label handling, `findIssuesLocalOnly`, and `fj v0.7.0` CLI options (`--style minimal issue search`, no `--json`).
- **Live Canary (`/usr/bin/fj`)**: Verified `fj --style minimal issue search --repo codeberg.org/ubunatic/harnez --state open|closed --all` outputs `N issue[s]` + `#<N>: <title> (by <user>)`, empty page returns `0 issues` with exit `0`, and nonexistent repo exits `1`.
- **Consult (`consult-1237`, `agy:neon:high`, 2m31s)**: Designed `NewFJFallbackSource` wrapper, exit-2 blocked search vs empty JSON safety, exit-0 zero-API `forge-assigned` `find issues next`, and stderr + inbox-draft label warnings.

## 2. Milestones & Review
- **M1 (`93281368`)**: `dev-1237` (`agy:neon:med`, 10m31s, 2.37M tokens incl. cached) added `internal/forgetracker/fj_search.go`, enabled `find issues next [--json]` on forge trackers (`NextResultJSON` `omitempty` extensions), added exit-2 private-repo blocked search reporting, added inbox `--label` warnings + draft blockquote note, and synced `help.golden` + `docs/commands/issue.md`. `make lint` and `HTO=0 make test-q1` passed.
- **Cross-vendor Review (`rev-1237`, `claude:haiku:high`, 1m58s)**: Returned **FAIL** on one blocking bug (`fj_search.go` lacked `--all` and `truncated` capping) plus minor items (distinguish configured token in blocked message, wire `SetFallbackStderr` on `new --forge`, sync `IssueTracking{,.lite}.md` & `config.yaml`).
- **Refinements (`2b5e5625`)**: `dev-1237` (`agy:neon:med`, 5m37s, 4.05M tokens incl. cached) fixed all findings and synced `docs/practices/IssueTracking{,.lite}.md`, `docs/IssueTracking.md`, `config.yaml`, and `.harnez/rules/Issues.md` via `./harnez init -d .`.
- **Re-review (`rev-1237-r2`, `claude:haiku:high`, 1m11s)**: Returned **PASS WITH MINOR FINDINGS**.
- **Host Verification**: `make install`, `make lint` (`0 issues.`, `lint: ok`), and `go test ./internal/forgetracker/... ./internal/find/... ./internal/claude/... ./cmd/harnez/...` (`ok` across all 4 packages).

## 3. Outcomes
- Private Forgejo repos without a harnez read token now fall back read-only to logged-in `fj` for `find issues`, `find issues similar`, and `issues new` duplicate gating (or cleanly exit 2 without prompting agents to debug token setup).
- `harnez find -d <forge-repo> issues next [--json]` succeeds (exit 0, 0 API calls) reporting `forge-assigned`.
- `issues new --label ...` inbox fallbacks on `fj` targets warn clearly on stderr and inside `issues/inbox/<slug>.md`.
