# 1022 — Add lmcoder coder and strata models through the x600 proxy

**Status**: Closed — M4 done in 09541733: provider_modes comment (manual default kept, no verified compaction for lmcoder); TestLmcoderCoderListedAsPiSupported now asserts StreamDriverFor maps lmcoder to PiStreamDriver, and a temporary guard made it fail. make test-q1: no --- FAIL, internal/subagent and internal/modelcatalog ok. lmcoder:coder live result from M3 stands. Parked: lmcoder:strata (out of scope); med/high tiers; --thinking low effect unverified; ModelSpec asymmetry in driver.go.
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
- Tier `low` with `effort: false` for `lmcoder:coder` (M2, host decision). The schema allows `low|med|high`, but the local model has no verified thinking levels yet. Enable `med` and `high` after the M3 live call.
- Pi model name is `coder`, not `lmcoder/coder` (M2, host decision). `piModelArgs` passes only the bare name, and `pi --list-models` shows one `coder` under provider `lmcoder`. M3 must confirm that Pi resolves the bare name.
- `DriverFor` maps provider `lmcoder` to `PiDriver` as well as `pi` (M2, host decision). Without it, the acceptance criterion "callable through Pi" fails, because `ModelEntriesWithDriver` lists the model as unsupported. The endpoint and key stay in `~/.pi/agent/models.json`, so nothing machine-specific enters the repo.

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
- M2 (coder catalog entry): done in `38190e98` and `98b5ef58`; reviewed by opus:low, approved with no blockers or majors.
- M3: Minimal live call to `lmcoder:coder` via the x600 proxy through Pi, or an honest blocker report.
  - Pre-work (from the M2 review):
    1. `spec/agent.yaml` (the `lmcoder:coder` entry, around line 239): the `use:` text names the host `x600`. Reword it to "the local lmcoder proxy" so no machine name appears in a shipped default.
    2. The Pi test expects `--thinking low` while the entry has `effort: false`. Confirm with a live call that Pi accepts `--thinking low` for this model. If Pi rejects it or the flag contradicts `effort: false`, drop the flag for this entry and report the change.
  - Run the live call with `harnez agent start` through Pi, using `lmcoder:coder`. Allow for the lazy model load on x600. Report the load time, the reply, and the error text if it fails.
  - Result (done, host run, 2026-10-29): `harnez agent start --role advisor --model lmcoder:coder` returned `Hello!` in 1 s with the model already loaded. The first direct Pi run (developer, before the probe) took 43.9 s, which includes the lazy load on x600; the two times are not split further. Pre-work 1 done in `f27aeebd`. Pre-work 2: Pi accepts `--thinking low` without error, but its reply reports thinking as `off`, so the flag's effect is unverified; it stays as-is for now.
  - Full suite after `f27aeebd`: `make test-q1` ran once with no `--- FAIL` and no make error; `internal/subagent` and `internal/modelcatalog` report ok.
- M4 (review pre-work for the lmcoder:coder diff, `38190e98^..HEAD`): the lmcoder:coder one-shot review (advisor role) requested changes with no blockers. Two findings to fix:
  1. `spec/agent.yaml:21` `provider_modes` lists `claude`, `codex` and `agy` but not `lmcoder`, so lmcoder falls back to the manual compaction default without anyone choosing it. Add an explicit `lmcoder` entry with the mode the Pi-served local model should use, or add a comment next to the `provider_modes` map that states why the default is right for lmcoder. Pick the mode by reading the existing `claude`/`codex` entries and their comments; if neither makes the choice clear, record the choice under `## Decision` and use the default with the comment.
  2. `internal/subagent/driver_test.go` `TestLmcoderCoderListedAsPiSupported` only calls the batch `DriverFor`. The `lmcoder` case in `StreamDriverFor` (`internal/subagent/driver.go`, about line 46) has no test, so removing it would not fail any test. Add an assertion that `StreamDriverFor` maps provider `lmcoder` to the Pi driver, next to the existing batch assertion. Keep the existing assertions unchanged.
  - Not in M4 (parked): the `driver.go` `ModelSpec` difference between the lmcoder and agy branches. It works today; it is recorded here, not fixed.
  - Acceptance for M4: `make test-q1` output has no `--- FAIL`; quote its final summary line; the new assertion fails when the `lmcoder` case is removed from `StreamDriverFor` (show that by a temporary local check, then revert it, and do not commit the revert change).
