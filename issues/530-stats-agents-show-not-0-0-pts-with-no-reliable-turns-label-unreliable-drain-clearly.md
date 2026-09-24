# 530 — stats --agents: show "—" not 0.0 pts with no reliable turns; label unreliable drain clearly

**Status**: Closed — 25a709b + 774e9f7: drain column carries (unreliable), totals show — without reliable turns
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: UX
**Related**: [[529-stats-agents-agy-turns-never-measured-and-deleted-check-sessions-missing]], `cmd/harnez/stats_agents.go`

## Problem

After 529 (2a7baf7), `harnez stats --agents --days 1 --all` on 2026-09-24:
- `agy:gemini-3.7-flash:med` rows show measured drains of 1–2 %, all marked
  unreliable, and the model total reads MEASURED TURNS 0, 5H DRAIN "0.0 pts". That
  reads as zero drain instead of no reliable data.
- "(unreliable)" is printed in the QUALITY column, but it qualifies the drain.

## /goal

A model total with zero reliable turns shows "—" for drain. The row output makes clear
which value is unreliable, without adding columns (narrow terminals).

## M1 delivered (25a709b), untested: host suite fails

The output is right ("2.0% (unreliable)" in 5H DRAIN, "—" in the totals), but
`TestRenderAgentStatsLabelsUnreliableDrainAndMissingModelDrain` fails at
stats_agents_test.go:268 ("unreliable marker should qualify drain, not quality").
The developer's single run stopped at gofmt, so the test never ran.

## M2 Pre-Work

- Fix the assertion at line 268 so it checks the rendered output correctly; the behaviour is correct.
- Run `gofmt` before `make test-q1`.
