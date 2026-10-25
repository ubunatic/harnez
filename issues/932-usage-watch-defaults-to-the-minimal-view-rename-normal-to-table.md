# 932 — usage: --watch defaults to the minimal view; rename --normal to --table

**Status**: Closed — 20213dac, ce2e9e93, 6f8873da: --watch starts minimal, --table selects the table; Remote Load title shows '@x600 · live' with no 'updated' row. Live tmux 160x50: box visible at start as '(@x600 · live)', r toggles it; make test-q1 green (dev run)
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

M1 delivered: 20213dac (minimal default, `--table`, hidden `--normal`); host tmux check of all four start modes OK.

## M2: Remote Load title shows freshness (owner, 2026-10-25)

Owner: label `@x600 · <num>m ago`, or `@x600 · live` when within the fast threshold; remove the separate
`updated ...` line; if that line also carries errors, show it only when there is an error.

- Title: `[R] Remote Load (@x600 · live)` while the sample age is within the live threshold, else
  `(@x600 · 12m ago)`; drop `server cache` from the title.
- Live threshold: a spec key (e.g. `remote_load_live_threshold`, default 2x `remote_load_poll_interval_attached`,
  i.e. 20s) with schema; no Go default.
- The `updated ...` body line goes away; an error line (stale reason, ssh failure) is shown only when there is one.
- Tests per cell for both titles and for the error line; update the 607 baseline golden (`remote-load-source.ansi`)
  in the same commit and say so.

M2 delivered: ce2e9e93 (title freshness, `updated` row removed, spec key `remote_load_live_threshold`).

### M3 Pre-Work (host review of M2, 2026-10-25)

- Host `make test-q1` after b0356c2a fails: `TestCollectorRecoversRemoteLoadForOneShot`
  (`cmd/harnez/usage_binary_test.go:509`, "one-shot usage lacks the recovered remote load box"). The output shows
  the box titled `(@x600 · 1h9m ago)`: the test still matches the old label, or the recovered sample's age is
  wrong (a just-recovered sample should read `live`). Find which, fix it, rerun the package, then `make test-q1`.
- Live, installed binary, tmux 160x50: `harnez usage --watch` shows `All Usage` and `[L] Load` but no Remote Load
  box, also after pressing `r`. Find out why (box hidden by default in minimal mode, `r` not bound there, or no
  cache read) and make `r` show it; report what the start state is.
