# 1196 — harnez find issues similar and issues new duplicate gate on forge tickets

**Status**: Closed — M1 and review refinements delivered (ef0130dc, 6bdd3bea): enabled find issues similar [--json] and issues new duplicate gate on forge trackers reusing find.Similar; make lint and touched Go packages passed.
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1195 (forge create), 1194 (/issue forge path), 1116 (forge show/find), 843 (local duplicate gate), 1118 (forge token)

---

/goal In a repo whose tickets live on the forge, `harnez find issues similar "<terms>"` scores forge tickets
with the same `find.Similar` as local tickets, and `harnez issues new` on the forge path refuses a strong match
unless `--force`, as the local gate does. Stop and report when blocked on a user decision; on a loop or no
progress, clear the goal and write a failure report here.

## Problem

Owner question (2026-11-09) after 1195. The local tracker blocks duplicates: `issues new` runs
`CheckDuplicates` (`internal/issues/actions.go`) with `find.Similar` (IDF-weighted title+body tokens,
thresholds and stopwords from the spec), `--force` overrides. On the forge path there is no gate, and
`similar` is in `findIssuesLocalOnly` (`cmd/harnez/issues_forge.go`), so agents fall back to title search.

## Want

- Map `forge.RemoteIssue` (number, title, body, state) to the input `find.Similar` takes; reuse its scoring and
  spec thresholds, no second scorer.
- `find issues similar` on the forge path; drop `similar` from `findIssuesLocalOnly`.
- `issues new` forge path: gate before running tea/fj/gh, `--force` overrides; `--dry-run` shows matches.
- Forge unreachable: warn and continue, like the open-PR number check.

## Notes

- Reads use the API token (`CODEBERG_TOKEN`, usually read-only, which is enough); creating uses the CLI's own
  login (tea/fj/gh), unchanged.
- The current list budget (`open_pr_scan`: 50 per page, 4 pages) covers about the newest 200 tickets per state.

## Open questions

- A separate, larger page budget in `spec/pr.yaml` for the duplicate scan? -> Decided below.
- Private repo without a token: fall back to `tea issues list` / `gh issue list --json` (CLI login)? -> Deferred to `#1237`.

## Decision

- Reuse `spec/pr.yaml` `issue_scan` (up to 10 pages x 20 = 200 issues) for forge `find issues similar` and the `issues new` duplicate gate instead of adding a new spec budget; if the scan result is `truncated`, print a brief note to stderr so callers know older issues were not fetched. (Host `/decide`, 2026-11-19.)
- Defer private-repo CLI fallback (`fj`/`tea`/`gh` list without a harnez read token) to `#1237` (next sprint) so API reads stay uniform here. (Host `/decide`, 2026-11-19.)
- On forge `issues new`, warn to stderr and continue when the forge read fails or is unreachable (matching `internal/issues/claims.go`), rather than failing closed offline. `--force` bypasses blocking on strong matches while still surfacing matches on `--dry-run`. (Host `/decide`, 2026-11-19.)
- Scope boundary: explicit `--inbox` offline drafts and `--remote` (`CLAUDE_CODE_REMOTE` GitHub flow) stay outside the forge duplicate gate here (`--remote` already documents its own flow; `--inbox` intentionally avoids forge calls). Direct unit tests calling `runIssuesNewForge` without `Source` document the nil-`Source` bypass rather than rejecting nil, keeping unit tests focused. (Host `/decide`, 2026-11-19.)

## Refinements (from `rev-1196` review of `ef0130dc`)

1. `printForgeTruncatedNote` (`cmd/harnez/issues_forge.go`): report `fetched` (`len(files)`) when `fetched > 0` instead of `PageSize*MaxPages`, so filtered PR pages do not overstate checked issues (`checked the newest %d forge issues; older issues were not fetched`).
2. `checkForgeNewDuplicates` (`cmd/harnez/issues_forge.go`): wrap `err` with `forgeTrackerHint(err, opt.Target)` in the skipped-check stderr warning so private-repo 404/401 hints surface consistently, and add a brief doc comment explaining when `opt.Source == nil` is valid (test callers that omit forge reads; CLI entrypoints always set `Source`).
3. `docs/commands/issue.md`: reword the `--dry-run` note to accurately say strong matches refuse unless `--force`, and `--dry-run` runs the check without writing (`--dry-run --force` surfaces matches without writing).
4. Tests (`cmd/harnez/issues_forge_test.go`): test `issues new` truncation note on stderr, unreachable host error (`forge.APIError{Status: 0, Transient: true}`), and `stdout`/`stderr` separation on `issues new --dry-run` duplicate output.

## M1 — `find issues similar` and `issues new` duplicate gate on forge trackers

- Reuse existing `find.Similar`, `spec/issues.yaml` thresholds/stopwords, and shared duplicate checking (`internal/issues/actions.go`, `internal/find/similar.go`); no second scorer.
- Write set (expected): `internal/forgetracker/` (map `forge.RemoteIssue` -> `issues.IssueFile`, similarity corpus helper), `internal/issues/actions.go` (shared duplicate check helper / forge-aware hint formatting), `cmd/harnez/find.go`, `cmd/harnez/issues.go`, `cmd/harnez/issues_forge.go`, `cmd/harnez/issues_duplicates.go`, `cmd/harnez/testdata/help.golden`, `docs/commands/issue.md`, and unit tests.
- Remove `similar` from `findIssuesLocalOnly` (`cmd/harnez/issues_forge.go`) and update `cmd/harnez/find.go` help + `help.golden`.
- `harnez find issues similar "<query>" [--json]` on a forge repo (including `issues.tracker: none` + `issues/inbox/`) fetches forge issues with `spec.IssueScan`, runs `find.Similar`, and renders matches (text and `--json`, reusing `find`'s renderer). If `truncated`, warn concisely on stderr.
- `harnez issues new` on the forge path (`--forge`, missing `issues/`, or `tracker: none` without local tickets) runs the duplicate check in `runIssuesNewForge` before `--dry-run` / creation:
  - Strong match without `--force`: refuse with non-zero exit (matching local `DuplicateError` style, with forge commands like `issues show [--forge] N` / `issues reopen [--forge] N` in hints) and make zero create calls (no API write, no CLI call, no inbox write).
  - `--force`: proceed with creation (and on `--dry-run`, display matches and planned create action).
  - `--dry-run` without `--force`: surface duplicate matches clearly and perform zero create calls.
  - Unreachable forge / transient API error: print a warning to stderr and continue to create / dry-run.
- Locked tests (`httptest` fake Forgejo + stubbed CLI/token lookup; remember `cmd/harnez/main_env_test.go` stubs `issueCLIRun`):
  1. `forgetracker` test mapping `forge.RemoteIssue` (number, title, body, state, URL) -> `issues.IssueFile` and verifying `find.Similar` scores open and closed forge issues with existing thresholds.
  2. `find -d <forge-repo> issues similar "<query>"` and `--json` return ranked matches (no longer rejected by `forgeLocalOnlyError`).
  3. `issues new` on forge path (both API write-token path and `fj` CLI path) with a strong match: refused without `--force` (assert API POST and `issueCLIRun` are NOT called); succeeds with `--force`.
  4. `issues new --dry-run` on forge path shows strong match and makes no write calls.
  5. `issues new` when forge listing fails (500/unreachable): warns on stderr and still creates (API/CLI/inbox as configured).
  6. Help golden & `golangci-lint` (`make lint`) + `HTO=0 make test-q1` pass.
