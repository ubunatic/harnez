# 1182 — usage: show Strata TPS box when the endpoint is online

**Status**: Closed — Strata TPS box in usage minimal/compact watch: spec keys strata_endpoints/timeout/ttl, /metrics reader, hidden when offline (unit-tested with httptest: live, idle, unreachable, malformed). Host live tmux 160x50 on installed binary: box 'qwen3.8-flash-next-iq2_xs 33.4 tok/s'. Live offline run not done (would stop the user's server); no toggle key; coder 8081/x600 out of scope
**Closed-By**: session 148f0fbb-3582-473b-96b3-e12d17e1d9a2
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

## Searcher findings (2026-11-06, live)

- Strata `GET http://localhost:8080/metrics` is JSON (not Prometheus), no flag needed: `live.tok_s` (null while
  prompt-phase), `requests[0].decode_tok_s` (last finished request, ~52), `totals`, `hardware.tok_s`. `/props` gives
  `model_alias` and `n_ctx` (label). `/slots` has no timings: do not use it.
- Panel wiring: `internal/usage/watch_frame.go` (panel select ~158-180, fetch ~417, append ~465), builders like
  `buildMicBox`, `buildLoadBox` (`watch_load_box.go`), `buildPowerBox` (`power_box.go`). Best pattern for a network
  endpoint: remote-load shape (`collector_remote_load.go`, `collector.go:121-142`: poll, cache, spec timeouts,
  `spec/usage.yaml:42-51`); hide the box when unreachable (Mic: `watch_frame.go:465`).
- Spec rule: add keys to `spec/usage.yaml` AND `spec/schemas/usage.schema.json`, not Go defaults. Tests: see
  `miclive_test.go`, `cmd/harnez/usage_binary_test.go`, `actionsspec_test.go`. `internal/subagent/driver_test.go`
  asserts `lmcoder:strata` is out of scope: leave it green, do not touch the model catalog.

## Decision

- Source `/metrics`; show `live.tok_s` while generating, else last `requests[0].decode_tok_s` dimmed with "idle".
- Config: spec key `strata_endpoints: [http://localhost:8080]` (list, default one entry), 1s timeout, cache like
  remote load; box hidden when no endpoint answers. Label from `/props` `model_alias`. Local host only (as Mic).
- Single milestone M1 (Strata TPS box); coder port 8081 and x600 remote stay out of scope (list entries allow it).
