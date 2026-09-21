# Session Report: Lean Sprints, Telemetry Data Management (2026-09-21)

Two lean-sprints ran in one session. Dev agent: `codex:luna:low` via `harnez agent start`.
Host wrote no code; it reviewed diffs, ran `make test-q1` once per change, and committed.

## Sprint 1: Now/Next items

| Ticket | Result | Commit |
|--------|--------|--------|
| 454 | Explicit `provider:model:tier` dispatched exactly; unknown specs fail with the known list, no silent fallback | see git log |
| 315 | Canary doc no longer hard-references the optional PrototypingFeatures doc | see git log |
| 442 | `issues open --commit` also publishes already-Open new tickets | `6073807` |
| 374 | README refreshed against current CLI help | `2d14641` |

450 was skipped on purpose: the fix belongs in `../loom` first.

## Sprint 2: Telemetry data quality

| Ticket | Result | Commit |
|--------|--------|--------|
| 424 | Fresh, legacy and current-version DBs all get `compaction_events.model`; a DB stamped current but missing the column now self-heals | `a1ade4a`, closed `6731cc2` |
| 425 (M1) | Migration report is accurate; `harnez apply` is silent on a no-op schema check | `235d7da` |
| 428 (telemetry part) | Authentic v2 and v5 upgrade fixtures with exact column assertions; apply logging tests | `235d7da` |
| 457 | Filed: canonical analytics queries as tests and live data-quality checks | `8240e37` |

Open in 425: Braille layout (M2). Open in 428: ANSI 256/truecolor.

## How the escalation ladder went

- Plan-first worked. On 424 luna's plan found the bug was already mostly fixed and named the
  remaining gap (DB stamped current but missing the column), which the host approved into scope.
  On 425/428 its plan showed three of the items were already done, so no work was wasted.
- One luna miss: the current-version test fixture lacked `tool_calls.ticket_id`, so schema
  creation failed before the repair ran. Fixed by resuming the same luna session with the exact
  error; no escalation to sol, Sonnet or Opus was needed in this sprint.
- Workers cannot commit or run `make test-q1` (read-only `.git`/quota state in their sandbox).
  The host ran tests and committed after reviewing each diff.

## Review notes for follow-up

- The 425/428 worker rewrote `TestEnsureTelemetrySchemaMigratesBeforeApply` so it no longer
  stamps the DB stale first. It now covers the no-op path; the migration path is covered by
  `TestApplyCmdMigratesLegacyCompactionSchema`. The test name is now misleading.
- The worker also made `apply` skip the session-tip DB reads in `cmd/harnez/main.go`, so the
  tip hook cannot open the DB ahead of the migration. Behavior looks intended, but the ticket
  did not ask for it.

## Next

Recommended order: 341 (reproduce first), 127 (SQL to `spec/`), then 457 built on 127,
then 446 before 445. See `docs/Roadmap.md`.
