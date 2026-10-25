# 915 — issues close records model and session provenance

**Status**: Open
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

## M2 — Pre-Work / Required Refinements (host, 2026-10-25)

Test isolation bug found by 945's developer run: inside an agent session `HARNEZ_AGENT_MODEL` is set (here
`codex:sol:low`), so older close tests get an unexpected `**Closed-By**: codex:sol:low` line and fail:
`TestIssuesCloseAmendReason/closed` and `TestIssuesCloseBatchPreservesClosure/{mixed,amend}`
(`cmd/harnez/issues_close_test.go:174,233`). Host runs pass because the variables are unset there.

1. Clear `HARNEZ_AGENT_MODEL` and `HARNEZ_SESSION_ID` for every test that does not set them on purpose (package
   `TestMain` in `cmd/harnez` and `internal/issues`, or the shared fixture helper); check other packages that
   run `issues close`.
2. Regression check: run the failing tests with both variables exported to non-empty values; they must pass.
3. `issues open` (reopen) removes the `**Closed-By**` line; an open ticket must not carry one (seen on 915
   itself after reopen). Test it.