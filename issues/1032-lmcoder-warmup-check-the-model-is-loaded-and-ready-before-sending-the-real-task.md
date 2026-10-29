# 1032 — lmcoder warmup: check the model is loaded and ready before sending the real task

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Enhancement
**Related**: 1024, 1029, 1030, lmcoder 263

---

## Ask (owner, 2026-10-29)
When harnez calls an lmcoder model, it first asks for a quick live sign (or reads a metrics endpoint) to see when
the model is loaded and the prefill is done. Only then does it send the real task.

## Why
The lmcoder proxy loads models lazily: the first request can take about 30 s while the model loads, and on the
larger window a long prefill now takes 100 s or more (owner, 2026-10-29). During that time a harnez run shows
nothing, so a slow start looks the same as a hang, and a server restart by another agent surfaces only as a
late provider error (1024 attempts: `engine stopped unexpectedly (exit code -15)`, then `502`).

## Known signals
- `GET http://127.0.0.1:8735/v1/models` lists each model with `lmcoder.live` and `lmcoder.live_n_ctx`. It does
  not load anything.
- Not checked: whether the proxy or the Strata engine has a health, metrics or progress endpoint (prefill
  progress, queue), and whether a 1-token chat request is a cheap enough live sign.

## Open questions
- What "ready" means: model loaded (cold start), or also the fixed prompt prefix prefilled so the real task
  reuses it from the prompt cache (lmcoder 250 saw cached prefixes cut 28 s to 4-7 s).
- Wait limits and output: show a "waiting for lmcoder model to load" line with elapsed time; fail with a clear
  message after a spec-configured limit; retry once after a restart (owner rule for this server).
- The probe must only touch the requested model: never load another one (never `strata` unless asked).

## Acceptance
- A run on a cold `lmcoder:coder` prints the warmup state and sends the task only after the live sign.
- A down or restarting server gives a clear "lmcoder not ready" error before Pi is launched.
- Tests with a fake proxy (cold, loading, ready, down); limits in `spec/` with a schema.

/goal Add an lmcoder warmup step that waits for a live sign before the real task, or stop and report when the
server offers no usable signal and an lmcoder-side change or an owner decision is needed. Circuit breaker: on a
loop or no progress, clear the goal and write a failure report. Re-check the proxy's API first; written 2026-10-29.
