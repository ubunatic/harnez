# harnez agent start: warn on high model quota using fresh, timestamped usage data only

Status: Open
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
