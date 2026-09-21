# Session Report: Lean Sprints, Telemetry Data Management (2026-09-21)

Two lean-sprints ran in one session. Dev agent: `codex:luna:low` via `harnez agent start`.
Host wrote no code; it reviewed diffs, ran `make test-q1` once per change, and committed.

## Sprint 1: Now/Next items

| Ticket | Result | Commit |
|--------|--------|--------|
| 454 | Explicit `provider:model:tier` dispatched exactly; unknown specs fail with the known list, no silent fallback | `0e82504`, `cd4286b`, `430df7c` |
| 315 | Canary doc no longer hard-references the optional PrototypingFeatures doc | `52d7363` |
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

## Sprint 3: Store hardening and canonical checks

| Ticket | Result | Commit |
|--------|--------|--------|
| 341 | Reproduced on Linux with a 32-process cold-start test (`duplicate column name: model`, one lost row). Fixed by running schema init and migration in one `BEGIN IMMEDIATE` transaction. Ticket stays open until a real macOS CI run passes | `fbaa511` |
| 127 M1 | Schema DDL and every `INSERT` moved to `spec/telemetry.yaml`, with a loader, JSON schema and an AST test that forbids SQL literals | `c812720` |
| 127 M2 | All `query.go` SELECTs, named filter predicates and a grouped-column allowlist moved to the spec; a test fails on dead spec entries | `d5178e1` |
| 457 | Six spec-defined quality checks with fixture tests, plus `harnez stats --quality` (`--json`, `--strict`), read-only. Live run over 32,208 tool calls: all PASS. Closed | `9bedd31`, `206e9d6` |

127 stays open for the remaining SQL (`telemetry.go` migrations, `classify.go`, `sanitize_cache.go`,
`issuesnapshot.go`, `export.go`, `economics_query.go`).

**Finding from 457:** `tool_calls` has no `model` column, so "calls per model" analytics are
impossible today. It belongs next to the 446/445 cost work.

## Ladder notes for sprint 3

- Still no escalation beyond luna:low. Every miss was a small, fixable defect that luna fixed after
  a resume with the exact error: an incomplete test fixture (341), a `const` that became a runtime
  value (127), a test matching doc comments (127), a wrong relative path (127), and two spec
  entries that were never wired into Go (127 M2, caught by review of the diff).
- Recurring pattern: workers cannot run the full suite, and their filtered `-run` patterns missed
  tests they had just added. The host's single `make test-q1` per change caught all of these.
  Worth putting into the worker prompt: "verify with the whole package, not a filtered run".
- Plan-first paid off again: the 341 plan flagged that `Open()` already had retries, and the 127
  plan found the ticket's inventory was stale (8 query statements, not 5) and other files with SQL.
- A pre-existing uncommitted edit to `docs/AgenticLoop.md` (steered escalation and session-record
  rules) was left out of every commit; it is not part of this sprint's work.

## Next

341 needs a macOS CI run to close; 127 needs a scope decision on the remaining SQL. Then 446
before 445, then 124 with 296. See `docs/Roadmap.md`.

## Hygiene and 462 (later in the session)

- Closed 341 (Linux-verified only), 127, 425 and 428, and filed follow-ups 458-462. 462 was decided
  by the user (drop `warn_condition`, keep the `apply` tip skip) and delivered by luna:low in one
  pass (`1aa05ed` closes it).
- The 462 worker skipped the plan-first step and reported only after finishing. The diff was small
  and matched the ticket, so nothing was lost, but the prompt's "reply with a plan before editing"
  is not enforced by `agent start` in one-shot mode. Plan-first needs a two-step start (plan-only
  prompt, then `resume` to grant write authority) to be real.
- The worker reported two failing `exec_test.go` tests as pre-existing. They passed under the
  host's `make test-q1`, so they are sandbox artifacts (read-only quota/cache state), not repo bugs.

## Learnings for the evergreen docs

- Store internals (serialized migration, spec-defined SQL, quality checks) now live in
  `docs/Telemetry.md` section 5, items 5-7.
- Worker-loop lessons (resume with the exact error, whole-package verification, host runs tests and
  commits) are recorded here only; `docs/AgenticLoop.md` has an uncommitted user edit and was not
  touched.
