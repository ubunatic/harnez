# harnez agent start: warn on high model quota using fresh, timestamped usage data only

Status: Closed — Implemented timestamped pre-turn quota notices from shared readings in 258ad24d; all required checks passed, live check reserved for host
Priority: P2
Category: Feature

## Problem
q-harnez (opus) died mid-turn when the Claude 5h window ran out (2026-10-17), leaving untested
WIP. The host checked `harnez usage` by hand between rounds.

## Requirements (user decision)
- Warn only when the limit reading is fresh; never act on stale data.
- Every warning carries the reading's timestamp and age, e.g.
  `quota: claude 5h 86% (read 10:41:07, 40s ago, resets 11:52)`, so agents can reason about
  actuality.
- If the reading is older than a spec threshold (spec/usage.yaml), say so instead of warning
  (`quota: claude 5h unknown, last reading 23m old`), or refresh first when cheap.

## Acceptance
- start/resume print the timestamped line when the provider's window is above a spec threshold
  (e.g. 80%); no warning from stale data; tests with fake clock and cache ages.

## Delivery

- Implemented in `258ad24d` (`feat(agent): warn on fresh high quota before turns (#819)`).
- Start/resume reuse the existing shared-store availability reader and its per-window
  timestamps; no second reader, provider probe, or wait for the concurrent capture.
- Spec and schema own the inclusive 80% threshold and inclusive 2m maximum age.
  Both 5h and weekly windows appear on one stderr line when applicable; stale
  windows show `unknown`, and missing/below-threshold fresh readings are silent.
- Fake-clock/cache-age tests cover Claude, Codex, all three AGY model pools,
  threshold/age boundaries, stale/missing/future data, and mixed-age windows.
  Driver tests verify output precedes start/resume and stdout remains valid JSON.
- Updated `docs/HarnezAgentArchitecture.md`; help/flags are unchanged.
- Passed: `make lint`, `go test ./internal/usage/ ./cmd/harnez/`, `make test-short`,
  `make install`. Lint reported warnings only. Live agent check reserved for host;
  no `harnez agent` command was run by this worker.

```text
quota: claude 5h 86% (read 10:41:07, 40s ago, resets 11:52)
quota: claude 5h unknown, last reading 23m old (read 10:18:47)
```
