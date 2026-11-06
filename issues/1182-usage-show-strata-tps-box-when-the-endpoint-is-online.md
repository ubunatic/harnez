# 1182 — usage: show Strata TPS box when the endpoint is online

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: 765, 932, 1022

---

/goal `harnez usage --minimal` and `harnez usage --watch` show a new box with the current tokens per second (TPS)
of the local Strata server whenever its endpoint is online, next to the existing boxes, and hide it when the
endpoint is offline; verified live against a running Strata and with it stopped; or stop and report when blocked on
a user decision or denied permission. Circuit breaker: on a loop or no progress, clear the goal and write a failure
report here.

## Notes (owner, 2026-11-06)

- Request: "`harnez usage --minimal|-w` should show current TPS of strata if the endpoint is online, as a new box
  next to the others."
- Strata is the lmcoder-managed llama-style server (default `http://localhost:8080/v1`, coder on 8081; also proxied
  via lmcoder on 8735). Status and port: `lmcoder strata status`. Context was raised to 96k on 2026-11-06.

## Open questions (verify against live code and server first)

- TPS source: llama-server `/metrics` (needs `--metrics`) or `/slots` timings vs. `strata monitor`; pick what is
  already exposed, no server changes unless unavoidable.
- Endpoint config: spec-driven URL(s) with default `localhost:8080`; also cover the coder port and remote x600?
- "Current" TPS when idle: show last-run value dimmed, or `idle`.
- Follow the box conventions of 244 (Mic, hidden when absent) and 765 (spec keys, short timeouts, cache).
