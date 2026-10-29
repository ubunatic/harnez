# 1022 — Add lmcoder coder and strata models through the x600 proxy

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 942

---

## Goal

/goal Make `lmcoder:coder` available in the Harnez model catalog and callable via the lmcoder proxy on x600 (`lmcoder:strata` is out of scope, see Decision); verify both model selection and actual calls, or stop and report if blocked on user input, a denied permission, or unavailable infrastructure. On a loop, hard architectural or OS blocker, or no progress, clear the goal and write a failure report and resume point.

## Acceptance

- Recheck the live catalog, supported agent drivers, proxy configuration and recent history before implementation. Determine how the two names map to the proxy and which harness can actually call them; do not assume catalog entries alone provide a working driver.
- List and resolve both names through the normal model selection path, respecting catalog schema and model policy. Keep machine-specific endpoints and credentials out of shipped defaults; use the project's configuration conventions.
- Verify selection and error handling with isolated tests, then make a minimal live call to each model via the x600 proxy when available. Report unavailable models or infrastructure honestly rather than treating listing as proof of access.

## Decision

- Driver: the model is called through Pi Agent (`internal/subagent/pi.go`), chosen by the owner on 2026-10-29. Do not add a different harness for this name.
- Scope (owner, 2026-10-29): only `lmcoder:coder`. `lmcoder:strata` is not part of this ticket and is not tried yet.
- No model switching: the coder model is loaded lazily on demand on x600 and served through the lmcoder proxy to other services, so this ticket must not change which model the proxy loads.
- Expect a first live call to be slow while the model loads on x600; allow for that in the timeout and report it as load time, not as failure.
- Models: only Claude models for every agent in this ticket. OpenAI (codex) and Gemini (agy) quota is exhausted on this machine (owner, 2026-10-29), so do not start luna, sol, terra, codex or gemini models.

## Milestones

- M1 (done, Searcher, read-only): premise verified.
  - Catalog: `models:` in `spec/agent.yaml:97`; schema `spec/schemas/agent.schema.json:180` requires `provider`, `name`, and `tier` (`low|med|high`).
  - Loader: `internal/modelcatalog/catalog.go` (`Load`); user policy file `~/.config/harnez/models.yaml` is absent on this machine.
  - Resolution: `internal/subagent/driver.go:322` (`ModelEntriesWithDriver`), `:394` (`ResolveModel`).
  - Pi: `internal/subagent/pi.go:24` (`PiDriver`), `:125` (`piModelArgs`) emits `--model <name>` and `--thinking <tier>` (`med` maps to `medium`). Pi takes the provider URL and key from its own `~/.pi/agent/models.json`, not from harnez.
  - This machine's `~/.pi/agent/models.json` already defines provider `lmcoder` at `http://127.0.0.1:8735/v1` (`openai-completions`) with model IDs `coder` and `strata`. The key is not recorded here.
  - The repo has no Pi `lmcoder` provider. The canary template defines only `llamacpp`; `scripts/agent-canary/bin/codex-launch` defines `lmcoder`.
  - Not verified: whether `127.0.0.1:8735` reaches x600 and answers, and whether Pi needs `coder` or `lmcoder/coder` as the model name.
- M2: Catalog entry for `lmcoder:coder` in `spec/agent.yaml` with schema, resolved through the normal path, with isolated tests.
- M3: Minimal live call to `lmcoder:coder` via the x600 proxy through Pi, or an honest blocker report.
