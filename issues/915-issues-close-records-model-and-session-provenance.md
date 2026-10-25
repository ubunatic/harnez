# 915 — issues close records model and session provenance

**Status**: Closed — issues close --model/--session (env HARNEZ_AGENT_MODEL/HARNEZ_SESSION_ID, set by agent start) writes one Closed-By line under Status, replaced on re-close, hidden from indexes (e4ffd8f5). Developer make test-q1 green; host diff review and live close.
**Closed-By**: claude:opus-5-5, session 9ec5fca3
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: 904 (Recommendation 5), 911 (epic)

---

/goal `harnez issues close` accepts `--model <spec>` and `--session <id>`, writes them into the ticket's close
metadata so multi-provider attribution survives, with tests; or stop and report on a design choice (where the
fields live, whether the generated index shows them). Circuit breaker: on a loop or no progress, clear the goal
and write a failure report here.

Split out of 904 by epic 911: 904 keeps the doc-only skill changes. Check first whether `harnez agent` telemetry
already links a session to its ticket close, so the flags may default from it.

## Decision (host, epic 939, 2026-10-25)

No telemetry link exists today. `--session` defaults from `HARNEZ_SESSION_ID` (set by `harnez agent start`,
`internal/subagent/interactive.go:165`, `agy.go:40`); `--model` defaults from an env var that `agent start` sets
alongside it (add one if missing), else the flag. Both are written as one metadata line under Status
(`**Closed-By**: <model>, session <id>`), omitted when both are empty; the generated index does not show them.
Reversible.