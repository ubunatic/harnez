# 1196 — harnez find issues similar and issues new duplicate gate on forge tickets

**Status**: Open
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

- A separate, larger page budget in `spec/pr.yaml` for the duplicate scan?
- Private repo without a token: fall back to `tea issues list` / `gh issue list --json` (CLI login)?
