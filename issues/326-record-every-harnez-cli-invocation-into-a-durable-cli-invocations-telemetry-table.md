# 326 — Record every harnez CLI invocation into a durable cli_invocations telemetry table

**Status**: Draft
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: issues/327 (`harnez log` read command), issues/328 (agent-vs-human attribution), issue 183 (sessionstate), issue 228 (`issue_status_snapshots`), `docs/CLIDesign.md`

---

## 1. Problem & Motivation

harnez has three history-shaped stores today, and **none of them records "which harnez
subcommand ran, when, where, and did it succeed"**:

- `tool_calls` (`internal/telemetry/schema.go`) records *tool* calls — rows written by
  `harnez rate` (an agent rating an internal tool call) and `harnez exec` (a wrapped shell
  command). A `harnez apply` or `harnez init` run writes no row of its own.
- `issue_status_snapshots` (issue 228) records only ticket *counts*, one row per
  `harnez index` run, and only when the counts differ from the previous row
  (`InsertIssueSnapshot`'s Go-side dedupe in `issuesnapshot.go`). It is a content-state
  timeline, not an invocation log — three `index` runs with identical counts leave one row
  or none.
- `internal/sessionstate` **does** see every invocation: `sessionTipHook` is wired as the
  root command's `PersistentPreRunE` (`cmd/harnez/main.go:143`) and calls
  `sessionstate.Record(&s, cmd.Name(), now)` on every subcommand. But it persists only
  `map[subcommand]{Count, LastAt}` into an ephemeral per-session JSON file under
  `~/.harnez/sessions/<hash>.usage.json`. There is no project, no working dir, no exit
  code, no duration, no per-invocation row, and no query path — the files accumulate and
  are only ever read back by the gap-tip heuristic.

So the question "what has harnez actually done in this repository, and when?" is currently
unanswerable. Repo *content* history (docs and issues) is already recoverable from git
(`git log -- docs/ issues/`, plus `harnez dochistory` and `harnez find issues history`), but
CLI *invocation* history exists nowhere durable.

This ticket covers the **write path only**. The read command is issues/327.

## 2. Technical Specification / Findings

### Proposed table (purely additive)

Add to `schemaDDL` in `internal/telemetry/schema.go`. Additive-only, so **no
`schemaVersion` bump** — same reasoning the `issue_status_snapshots` block in that file
already documents (`CREATE TABLE IF NOT EXISTS` applies it to a pre-existing DB on next
`Open`).

```sql
CREATE TABLE IF NOT EXISTS cli_invocations (
	id             INTEGER PRIMARY KEY AUTOINCREMENT,
	created_at     TEXT    NOT NULL,
	session_id     TEXT    NOT NULL,
	agent_id       TEXT    NOT NULL,
	command        TEXT    NOT NULL,          -- full cobra path, e.g. "issues new"
	args           TEXT    NOT NULL DEFAULT '', -- redacted; see privacy below
	project_name   TEXT    NOT NULL DEFAULT '',
	working_dir    TEXT    NOT NULL DEFAULT '',
	ticket_id      TEXT    NOT NULL DEFAULT '',
	exit_code      INTEGER,
	duration_ms    INTEGER NOT NULL DEFAULT 0 CHECK (duration_ms >= 0),
	harnez_version TEXT    NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_cli_invocations_created_at ON cli_invocations (created_at);
CREATE INDEX IF NOT EXISTS idx_cli_invocations_project    ON cli_invocations (project_name);
CREATE INDEX IF NOT EXISTS idx_cli_invocations_session_id ON cli_invocations (session_id);
CREATE INDEX IF NOT EXISTS idx_cli_invocations_command    ON cli_invocations (command);
```

## Why this needs real work, not a quick patch

- **`PersistentPreRunE` is the wrong hook for exit codes.** The existing
  `sessionTipHook` fires *before* the command body, so it can never observe `exit_code` or
  `duration_ms`. Cobra's `PersistentPostRunE` is also insufficient: it does **not** run when
  the command's `RunE` returns an error — exactly the case most worth recording. The write
  must therefore happen in `main()` around `root.Execute()`, capturing start time before and
  the returned error after, with the resolved command path plumbed out of the pre-run hook
  (or recovered via `root.Find(os.Args[1:])`). This is the single load-bearing design
  decision in the ticket and must be covered by a test asserting a failing subcommand still
  produces a row with a non-zero `exit_code`.
- **Writing on *every* invocation is a new hot path.** `sessionTipHook` already opens the
  telemetry DB per call (for `UnratedFailureCount`), so the cost precedent exists, but this
  adds a synchronous write to commands that currently touch no DB at all (`apply`, `init`,
  `status`, `diff`). It must stay strictly best-effort and never fail or block a real command,
  matching `sessionTipHook`'s stated "swallowed silently" contract. `internal/telemetry`'s
  existing benchmark (`telemetry_bench_test.go`, `rate_overhead_test.go`) gives a place to
  assert the added per-call overhead stays negligible.
- **`args` is a genuine privacy hazard, unlike anything in `tool_calls` today.** Argv can
  carry absolute paths, hostnames (`usage --host`), and ticket titles. `internal/telemetry`
  already has a multi-level export privacy model (`export.go`, `sanitize_cache.go`,
  `privacy_export_test.go`) — this column must participate in it rather than bypass it.
  Default to storing flag *names* plus non-sensitive positionals, and redact values for an
  explicit deny-list of flags; do not store raw argv verbatim by default.
- **Unbounded growth.** Unlike `tool_calls` (written only on deliberate `rate`/`exec`
  calls) and `issue_status_snapshots` (deduped), this table gets a row per invocation
  forever. It needs a retention story from day one: a row cap or age-based prune on write,
  and a documented opt-out env var alongside the existing `HARNEZ_DISABLE_RATE_FEEDBACK`
  convention.

## 3. Implementation & Verification Plan

### Proposed scope

- `internal/telemetry`: add the DDL block above, a `CLIInvocation` struct in `types.go`
  mirroring the columns (per that file's stated "mirrors schemaDDL exactly" rule), an
  `InsertCLIInvocation` in `insert.go`, and a `QueryCLIInvocations(Filter, limit)` reader
  (extend the existing `Filter` with a `Command` field rather than inventing a second
  filter type).
- `cmd/harnez/main.go`: capture start time, run `root.Execute()`, derive exit code from the
  returned error, and write the row best-effort afterwards. Reuse `resolve.Session`,
  `resolve.Ticket`, and `detectAgent` (`cmd/harnez/rate.go:43`) — all three already work
  without an agent present.
- Arg redaction helper with a flag deny-list, unit-tested independently of the command path.
- Retention prune (age or row cap) applied on write, plus `HARNEZ_DISABLE_CLI_LOG`
  (name TBD) opt-out honoured before any DB work happens.

### Verification

- [ ] A successful `harnez status` run writes exactly one `cli_invocations` row with
      `exit_code = 0` and a non-zero `duration_ms`.
- [ ] A **failing** subcommand (one whose `RunE` returns an error) still writes a row, with
      a non-zero `exit_code` — asserts the `PersistentPostRunE` trap above is avoided.
- [ ] `command` records the full subcommand path (`issues new`, not `new`).
- [ ] A run with no telemetry DB present, an unwritable DB, or an unresolvable session id
      succeeds normally and prints nothing extra — best-effort contract holds.
- [ ] Redaction test: a deny-listed flag's value never appears in the stored `args`.
- [ ] Retention test: writing past the configured cap/age prunes the oldest rows.
- [ ] Opening a pre-existing schemaVersion-2 database creates the new table without a
      version bump or migration error.
- [ ] `go test ./...` passes; per-call overhead measured against the existing
      `rate_overhead_test.go`/`telemetry_bench_test.go` baselines.
