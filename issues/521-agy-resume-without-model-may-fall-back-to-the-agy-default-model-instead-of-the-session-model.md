# 521 — agy resume without --model may fall back to the agy default model instead of the session model

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Bug
**Related**: 497, 498, `internal/subagent/agy.go`, `cmd/harnez` agent resume

## /goal

Resuming a harnez agent session always runs on the session's recorded
provider model and tier unless `--model` overrides it. Confirm or refute this
with a canary, and fix the resume path if the recorded model is dropped.

## Observation (loom 107 lean sprint, 2026-09-24)

- Session `dev107f` was started with `--model agy:flash37:med`; M1 worked.
- `harnez agent resume --name dev107f "<prompt>"` (no `--model`) failed:
  `agy resume: Individual quota reached ... Resets in 90h48m43s`.
- At that time the agy default model was Sonnet, at 100% quota. flash37 had
  quota left. This suggests resume ran on agy's default model, not on flash37.
- `AgyDriver.Resume` passes `--model`/`--effort` only when `model.Name != ""`.
  Otherwise agy uses its own default. 497 closed with "tier passed on exec and
  resume (session tier)", so either the session model isn't reaching the agy
  driver, or agy `--conversation` ignores it.

## Investigate

- Canary: set the agy default to model A, start a session on model B, resume
  without `--model`, and check which model served the turn (agy output or
  telemetry).
- Check whether the resume command fills `Model` from the stored session for
  agy, as it does for codex (497).
- The quota error should name the model that hit the limit.
