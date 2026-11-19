---
kind: sprint
ticket: 1196
date: 2026-11-19
branch: sprint/taia-harnez-pack
---

# Two-Agent Lean Sprint: 1196 — Forge `find issues similar` & `issues new` Duplicate Gate

## 1. Scout Audit (`search-1196`, `claude:haiku:med`)
- Located `find.Similar` (`internal/find/similar.go`), `issues.CheckDuplicates` (`internal/issues/actions.go`), `checkIssueDuplicates` (`cmd/harnez/issues_duplicates.go`), `forgeLocalOnlyError` (`cmd/harnez/issues_forge.go:34`), and `runIssuesNewForge` (`cmd/harnez/issues_forge.go:421`).
- Confirmed `spec/pr.yaml:16-19` `issue_scan` covers up to 200 items (`10 x 20`) and `internal/issues/claims.go` warns-and-continues when the forge is unreachable.

## 2. Milestones & Review
- **M1 (`ef0130dc`)**: `dev-1196` (`agy:neon:med`, 13m58s, 3.4M tokens incl. cached) mapped `forge.RemoteIssue` to `issues.IssueFile` in `internal/forgetracker/forgetracker.go`, shared duplicate scoring via `CheckDuplicatesFromFiles` / `DuplicateErrorWithOptions` (`internal/issues/actions.go`), enabled `harnez find issues similar [--json]` on forge trackers, gated `harnez issues new` on the forge path before `--dry-run`/create, updated `help.golden` and `docs/commands/issue.md`, and passed `make lint` + `HTO=0 make test-q1`.
- **Cross-vendor Review (`rev-1196`, `claude:haiku:high`, 2m9s)**: Returned **PASS WITH MINOR FINDINGS** (truncation banner used budget vs fetched count, skipped-check warning lacked `forgeTrackerHint`, doc `--dry-run` wording, and stream/unreachable-host test coverage).
- **M1 Refinements (`6bdd3bea`)**: `dev-1196` (`agy:neon:med`) reported fetched count (`len(files)`) when truncated, wrapped skipped warnings in `forgeTrackerHint`, clarified `docs/commands/issue.md`, and added stderr/stdout split, unreachable-host, and private-repo 404 hint tests.
- **Host Verification**: `make install`, `make lint` (`0 issues.`, `lint: ok`), and `go test ./internal/forgetracker/... ./internal/issues/... ./internal/find/... ./cmd/harnez/...` (`ok` across all 5 packages).

## 3. Outcomes
- `harnez find -d <forge-repo> issues similar "<query>" [--json]` now scores open and closed forge issues via `find.Similar`.
- `harnez issues new` on forge trackers (`issues.tracker: none` or `--forge`) blocks strong matches unless `--force` is passed, shows matches on `--dry-run --force`, and warns-and-continues when unreachable.
- Private-repo CLI fallback (`fj`/`tea`/`gh`) without a read token remains scoped cleanly to `#1237`.
