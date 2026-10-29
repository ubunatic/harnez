# 1025 — lmcoder:coder follow-ups parked from 1022

**Status**: Closed — items 2 and 3: lmcoder 260 says all tiers are ignored, catalog already enables only low; item 1: owner treats strata like coder, compaction override already written by 1023 M1; item 4: asymmetry intended, ModelSpec only feeds agy's HARNEZ_AGENT_MODEL and PiDriver passes the model as arguments
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

## Result for items 2 and 3 (lmcoder 260, closed in lmcoder 33ce174, 2026-10-29)
- Verdict: `low`, `med` and `high` are all ignored by the coder server as Pi and the proxy are configured today
  (thinking is off; 107-140 output tokens, no reasoning, about 4 s each, identical prompt). With thinking turned
  on, low/medium think about 180 tokens in 11-14 s and high 1.7-3.2k tokens in up to 108 s.
- Recommendation from 260: harnez enables only `low` with `effort: false`. The catalog entry from 1022 already
  does exactly that, so items 2 and 3 are done with no code change: `med`/`high` stay disabled and
  `--thinking low` is a no-op on this server.
- Untested in 260: a real Pi run with `reasoning: true`, a `chat_template_kwargs` override, run-to-run variance.
  Enabling thinking would be a new owner decision, not part of this ticket.
- Items 1 (`strata`, parked until the owner asks) and 4 (`ModelSpec` asymmetry) remain open.

## Result for items 1 and 4 (2026-10-29)
- Item 1: owner note: treat `lmcoder:strata` like `coder` and file new issues only when a problem shows up.
  `harnez apply` already writes its Pi compaction override (1023 M1). Nothing more to do here.
- Item 4: the asymmetry is intended. `AgyDriver.ModelSpec` exists only to set `HARNEZ_AGENT_MODEL` for agy
  (`internal/subagent/agy.go:52-53`). `PiDriver` has no such field and passes the model as Pi arguments, like
  the claude and codex drivers, so the lmcoder branch has nothing to set.
