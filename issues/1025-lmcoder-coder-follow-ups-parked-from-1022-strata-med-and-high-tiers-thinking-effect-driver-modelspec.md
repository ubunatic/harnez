# 1025 — lmcoder:coder follow-ups parked from 1022

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: 1022, 1023, 1024, lmcoder 260

---

## Context

Ticket 1022 (closed) added `lmcoder:coder` to the model catalog and drives it through Pi. Four items were parked on purpose. Each one is listed here so it survives the close.

## Items

1. **`lmcoder:strata` (out of scope)**. The owner limited 1022 to `coder`. Do not add or try `strata` until the owner asks. The proxy loads one model at a time, so `strata` also needs a model-switching decision first.
2. **Tiers `med` and `high` are disabled.** The catalog entry enables only `low`, with `effort: false`. Enable `med` or `high` only after lmcoder issue 260 gives a verdict per tier, with evidence from identical prompts.
3. **Effect of `--thinking low` is unverified.** Pi accepts the flag, but its reply reports thinking as `off`. Until 260 shows whether the proxy acts on the level, treat the flag as a no-op.
4. **`ModelSpec` asymmetry in `internal/subagent/driver.go`.** The `lmcoder` branch in `DriverFor` does not set `ModelSpec`, unlike the `agy` branch. It works today. Fix it only if a test or a caller needs the spec. `StreamDriverFor` has no separate lmcoder case; it reaches Pi through `DriverFor`.

## Acceptance

- Item 1: stays out until the owner asks.
- Items 2 and 3: closed by a result from lmcoder 260, with the tier verdict recorded here.
- Item 4: either a test that proves the need, or a note that the asymmetry is intended.
