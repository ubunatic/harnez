# 932 — usage: --watch defaults to the minimal view; rename --normal to --table

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 920, 929, Epic 921

---

/goal `harnez usage --watch` opens the minimal view without a flag, and the usage-table-only view is selected with
`--table` (help text and docs say so); verified in tmux on the installed binary; or stop and report. Circuit breaker:
on a loop or no progress, clear the goal and write a failure report here.

## Notes (owner, 2026-10-25, quickfix)

- Owner: "I always use `harnez usage --watch --minimal` which should be the default I guess. The other mode where
  we only see the usage table may need another name."
- Today: `--normal` "show the usage table (the default view)", `--compact`, `--minimal` (compact panels without
  title/status bars or hints).

## Decision (host)

- Scope: `--watch` only. Plain `harnez usage` (one-shot print) keeps the table, the natural output for a pipe.
- `--table` selects the table view; `--normal` stays as a hidden alias so scripts keep working.
- `--minimal` stays accepted (now a no-op with `--watch`). The `[m]ode` key cycle is unchanged; the start mode
  changes only.
- Defaults live in `spec/` if the default view is a spec value today; otherwise where the flag default is.
