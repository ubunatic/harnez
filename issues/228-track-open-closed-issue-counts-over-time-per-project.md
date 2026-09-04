# 228 — Track open/closed issue counts over time per project

**Status**: Closed — resolved in ece33c7
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: [[036-harnez-status-issues-tracker-linter]]

## Problem

Issue status (Open/Closed/etc.) lives only as plain text in each
`issues/*.md` ticket header, rolled up by `harnez index` into
`issues/README.md`. That rollup is a point-in-time snapshot — nothing
retains history, so there is no way to see how many tickets were
open/closed per project at a given point in the past, or to chart
throughput/backlog trends over time.

## Scope

- On `harnez index -d <repo>` (or a lightweight companion step),
  append a dated snapshot row — timestamp, project/repo, open count,
  closed count (and optionally per-priority breakdown) — to a small
  append-only log (e.g. `issues/.status-history.jsonl` or a
  `stats_history` table in the existing telemetry SQLite DB under
  `internal/telemetry`, whichever keeps SQL/format ownership in one
  place per [[120]]'s "keep SQL out of the CLI" convention).
- Add a read path (`harnez find issues history` or a `harnez stats
  --issues` addendum) that renders the recorded snapshots as a simple
  table, filterable by project like [[227]]'s `--project`.
- Snapshot writes must be idempotent/cheap enough to run on every
  `harnez index` invocation without noticeably slowing it down, and
  must not fail `index` if the history write fails (log-and-continue,
  not command-failing).

## Acceptance Criteria

- [x] Running `harnez index` twice in a row with no ticket changes
      produces at most one meaningful new history entry (or clearly
      dedupes identical consecutive snapshots) rather than growing
      unboundedly on repeated no-op runs.
- [x] History read path shows open/closed counts per project across
      at least two distinct recorded points in time, verified against
      a seeded fixture.
- [x] Storage choice (flat file vs. telemetry DB) and its rationale
      documented in the ticket's Resolution.
- [x] `harnez index --help` (or the new read command's `--help`)
      documents the feature.

## Notes

Scope this to counts only (open/closed, optionally by priority) — do
not attempt full issue-level history/diffing (title/status-transition
audit trails) in this ticket; that would be a much larger, separate
feature if ever needed.

## Resolution

**Storage choice: a new `issue_status_snapshots` table in the existing
telemetry SQLite DB (`internal/telemetry`), not a flat
`issues/.status-history.jsonl` file.** Rationale: the telemetry DB
already owns every other piece of cross-session/cross-project
structured history this repo records (`tool_calls`, the sanitize/
category caches), already has the exact convention this table needed
(additive `CREATE TABLE IF NOT EXISTS` + indexes, no `schemaVersion`
bump — see `schema.go`'s comment on `note_category_cache`), and already
has WAL-mode concurrent-writer safety a hand-rolled JSONL append would
have to reinvent. Reusing it keeps all of this repo's SQL/format
ownership in one place per [[120]]'s "keep SQL out of the CLI"
convention, rather than splitting issue history into a second,
bespoke on-disk format with its own parsing/locking code.

Added `issue_status_snapshots` (`internal/telemetry/schema.go`):
`id, created_at, project_name, open_count, closed_count, draft_count,
unknown_count`, plus `project_name`/`created_at` indexes — purely
additive, no schema version bump. `internal/telemetry/issuesnapshot.go`
adds `IssueStatusSnapshot`, `(*DB) InsertIssueSnapshot` (dedupes
against that project's most recently recorded row by comparing all
four counts — a Go-side read-then-compare, not expressible as a DB
constraint — and reports whether it actually inserted), and
`(*DB) QueryIssueSnapshots(project string)` (oldest first, project
empty means unfiltered).

`internal/index/index.go` gained `StatusCounts(issuesDir)`, aggregating
the same ticket set `IssuesTable` renders into open/closed/draft/
unknown counts by canonical status category. `cmd/harnez/index.go`'s
`runIndex` now takes an `indexOptions{Dir, Check, DBPath}` (DBPath is a
test-only override, mirroring `statsOptions.DBPath`/
`execOptions.DBPath`) and calls a new `recordIssueSnapshot` after a
successful non-`--check` `UpdateIssuesReadme` run: it computes counts,
opens the telemetry DB, derives `project_name` as
`filepath.Base` of the repo root (`issuesDir`'s parent, resolved to an
absolute path — the same identity convention `rate`/`exec` use via
`filepath.Base(wd)`), and calls `InsertIssueSnapshot`. Every failure
path (`StatusCounts`, `DefaultDBPath`, `Open`, `InsertIssueSnapshot`)
is swallowed via the existing `debugLog` helper (`DEBUG=1` writes
`~/.harnez/debug.log`) and never returns an error to `runIndex`'s
caller, satisfying the "must not fail index" requirement. `--check`
never records a snapshot, matching its existing no-disk-side-effect
contract.

Read path: `harnez find issues history [--project <name>] [--json]`
(`cmd/harnez/find.go`), dispatched the same way `harnez find issues
next` already is (`args[1] == "history"`). Non-JSON output is a
`PROJECT / CREATED_AT / OPEN / CLOSED / DRAFT / UNKNOWN` table, oldest
snapshot first; `--json` round-trips `[]telemetry.IssueStatusSnapshot`
(empty slice, not `null`, when there's no data yet). `--project`
mirrors [[227]]'s `--project` filter identity (`project_name`).

Tests: `internal/telemetry/issuesnapshot_test.go` —
`TestInsertIssueSnapshot_RecordsAcrossTwoDistinctPoints` (two seeded
snapshots with different counts both come back, oldest first),
`TestInsertIssueSnapshot_DedupesIdenticalConsecutiveSnapshots` (3
repeated identical-count inserts stay at 1 row; a genuine count change
still inserts), `TestQueryIssueSnapshots_FiltersByProject`.
`cmd/harnez/index_test.go` —
`TestRunIndex_RecordsIssueSnapshotAcrossTwoPoints` (real `runIndex`
call against a fixture repo, ticket flipped Open->Closed between two
runs, both counts visible via `QueryIssueSnapshots`) and
`TestRunIndex_NoOpRunsDoNotGrowHistory` (3 consecutive `runIndex` calls
against an unchanged fixture leave exactly 1 recorded snapshot).
`cmd/harnez/find_test.go` —
`TestRunFindHistory_TableShowsBothRecordedPoints` and
`TestRunFindHistory_ProjectFilter`.

`go build ./...`, `go vet ./...`, and `go test ./...` all pass; `make
install` updated the installed `~/go/bin/harnez` binary. Verified live
against this repo's own real telemetry DB via `harnez index -d .`
followed by `harnez find issues history --project harnez`.
