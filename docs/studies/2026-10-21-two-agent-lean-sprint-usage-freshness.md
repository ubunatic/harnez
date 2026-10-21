# Two-Agent Lean Sprint: usage freshness (issue 839)

Date: 2026-10-21. Host: Claude Opus 5.5. Workers: `s839` (searcher, luna:low), `d839` (developer, luna:high).

## Searcher
Asked which timestamp the Updated column renders, the collector cadence and backoff, whether
hooks update freshness, and the live collector state. Found in 48s: Updated = last quota fetch,
15m cadence by design, and an unnoticed SQLite `disk I/O error (6410)` on every snapshot write.

## Milestones
- M1 (collector save error), `3c54af18`: `ProtectSystem=strict` made `/tmp` read-only, so SQLite
  had no temp dir (`SQLITE_IOERR_GETTEMPPATH`). Fix: `PrivateTmp=yes` in the unit, failing-first
  unit-template test. Errors existed since the unit's first start (00:46).
- M2 (5-minute cadence, user choice c), `a3d236a2`: collector cadence 5m, cache age capped at one
  cadence, AGY meter max age 1h -> 5m. Issue 112 auth backoff (30m) kept.

## Verification
`make test-q1` failed before and passed after each fix; `make install` ok. Host ran `harnez usage`:
all rows "just now" after M2.

## Observations
- Plan-first turn paid off: the developer confirmed the root cause from `/proc/<pid>/mountinfo`
  before writing.
- Open risk: AGY's meter max age dropping from 1h to 5m means up to 12x more live
  `agy -p "/usage"` calls, the pattern issue 112 suspected of triggering Google reauth. The
  developer did not flag it; watch for reauth prompts.
- Cost: searcher 58k new tokens; developer 3 turns, ~465k new tokens, ~12 min total.
