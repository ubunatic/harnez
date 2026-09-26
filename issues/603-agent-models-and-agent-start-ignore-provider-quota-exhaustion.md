# 603 — agent models and agent start ignore provider quota exhaustion

**Status**: Closed — M1-M5 delivered; M5 fixed stale-vs-unknown regression
**Priority**: P1 (High)
**Severity**: Major
**Category**: Bug
**Related**: 023 (harnez usage quota tracking), 104 (AGY quota collector)

---

## 1. Problem & Motivation
In a neus `/lean-sprint 19` run (2026-09-27), the host picked `agy:flash38:med` from
`harnez agent models` and `harnez agent start` accepted it and reported "Started agent".
The agy provider was out of quota. Nothing told the host: `agent models` has no quota
column, and `agent start` does not check quota before starting. The session was
stopped with 0 tokens used, so it never did any work. The user had to catch the mistake.

## 2. Technical Specification / Findings
- `harnez agent models` shows COST/EFF/SKILLS/ROLES but no availability or quota state.
- `harnez agent start` launched a detached session on an exhausted provider without a warning.
- Quota data already exists (`harnez usage`, the shared quota cache from 033, the AGY collector from 104).

## 3. Implementation & Verification Plan
- `agent models`: mark or hide models whose provider quota is exhausted, using the cached quota
  state (showing "unknown" when there is no data).
- `agent start`: fail closed, or at least warn loudly, when the chosen provider is known to be
  exhausted, and name cheaper available alternatives.
- Tests: a fake quota cache with agy exhausted gives the marker in `agent models` and makes
  `agent start --model agy:...` refuse (or warn).

## M2 — Pre-Work / Required Refinements (from neus feedback, 2026-09-27)

- Real `harnez agent models` shows `unknown (681h47m)` on every row, agy included, although agy is exhausted.
  The snapshot read is 28 days old. Check first that the resolver reads the cache the collectors write *now*
  (issue 598 moved caches to XDG paths); a reader on an abandoned path is the likely bug.
- Old data must read `stale (28d)`, not `unknown`; reserve `unknown` for no data. Use a compact age (`4m`, `3h`, `28d`).
- Verify against the live cache: agy must show `exhausted` when `harnez usage` shows it at 0 remaining.

- **M1 delivered (column + start guard)** `e4ffc6c`; **M2 delivered (live cache, stale age)** `cb8a4de`.

## M3 — Pre-Work / Required Refinements (host review of live output, 2026-09-27)

- Age mismatch: `harnez usage` shows the agy row "updated 36m ago", `agent models` shows `stale (1d)` for the same
  provider. Both must derive age from the same timestamp; find which one is wrong.
- Exhaustion rule is wrong: agy Gemini 5h window is at 100% used, yet flash37/flash38 are not `exhausted`. A pool is
  exhausted when **any** active (not yet reset) window it depends on is at 100%, not only when all are.
- Check the staleness threshold against the collector cadence: data that `usage` still shows as recent must not
  read as stale in `agent models`.
- Verify again with pasted rows from both commands.
- **M3 delivered (consistent age, any-window exhaustion)** `dc86ac5`. Live: all agy rows `stale (44m) exhausted`, matching `harnez usage` (Gemini 5h window 100%). Host test-q1 green. `harnez apply` done. Awaiting neus feedback.
- neus (2026-09-27): M3 matches its view (agy `stale (45m) exhausted`, luna/terra `available`).

## M4 — Pre-Work / Required Refinements (neus feedback)

- A window at 100% cannot recover before its reset time. If the cached window has a reset timestamp still in the
  future, treat the pool as `exhausted` and **block** `agent start`, however old the snapshot is. Staleness only
  downgrades to non-blocking once that reset time has passed (then show `stale (<age>)`, since usage may have reset).
- Show the reset in the cell, e.g. `exhausted (resets 1h45m)`.
- Test: stale snapshot with a future reset blocks; stale snapshot with a past reset does not.
- **M4 delivered (block until reset)** `d47c3f1`. Live: agy rows `exhausted (resets 1h38m)` / `(resets 19h6m)`; `agent start --model agy:flash38:low` refused with override flag and `codex:luna:low` alternative (host-verified). flash37 USE wording fixed (604). Awaiting neus confirmation.
- neus (2026-09-27): verified; agy rows exhausted with reset times, no agy developer role, `agent start --model agy:flash38:low` refused without creating a session.

## M5 — Regression (host, 2026-09-27)

- `go test ./internal/usage -run TestCachedAGYAvailabilityKeepsExhaustionUntilReset` fails consistently since shortly
  after `d47c3f1`: `stale_past_reset_becomes_stale` gets `unknown`, wanting `stale ... age 36m from usage meter`.
  It passed at commit time, so the test depends on wall-clock time or on ambient state (real cache / usage meter)
  instead of fixed fixtures. Inject the clock and isolate state (temp XDG dirs); keep the assertions.
- **M5 delivered (regression)** `84de49d`: expired meter windows were discarded before the availability check, so past-reset data read `unknown` instead of `stale`; now kept for availability only; agent-models tests isolated from the real HOME meter. test-q1 green.
