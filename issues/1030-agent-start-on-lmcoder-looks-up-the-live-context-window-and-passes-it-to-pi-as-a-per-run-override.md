# 1030 — Agent start on lmcoder looks up the live context window and passes it to Pi as a per-run override

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Enhancement
**Related**: 1023, 1024, 1031, 969, lmcoder 263

---

## Problem
Pi takes the context window of `lmcoder:coder` from the static `contextWindow` in `~/.pi/agent/models.json`. The
server's real window changes: 32k, then 64k, now 96k is planned (lmcoder 263, 2026-10-29), and each change needs
a hand edit of that file. When the file is behind, Pi compacts too early (wasted window); when it is ahead, Pi
overflows and stops with `length` (1023, 1024). At the time of writing the proxy reported
`live_n_ctx: 65536` for `coder` while the owner reported 96k as the new size.

The proxy already publishes the live value: `GET http://127.0.0.1:8735/v1/models` returns, per model,
`lmcoder.live` and `lmcoder.live_n_ctx` (only while the model is loaded; the proxy loads lazily on first request).

## Ask (owner)
`harnez agent start`/`resume` with an lmcoder model looks up the model's max context and passes it to Pi as a
config override for that run, so the run uses the real window without editing `models.json`.

## Open questions
- What Pi 1.4.0 accepts per run: a CLI flag or env for the context window, a `--models`/config file path, or only
  `<agentDir>/models.json` (a per-run `PI_CODING_AGENT_DIR` loses auth and sessions, see 1023 Decision).
- Where the max comes from when the model is not loaded yet (`live_n_ctx` absent): a configured max in the proxy's
  model list, a spec value in harnez, or a probe request. A lookup must not load another model (never `strata`).
- How the derived compaction override (1023, `harnez apply`) follows the per-run value, and how 1031's
  `--autocompact` combines with it.

## Acceptance
- A run on `lmcoder:coder` uses the proxy's window, shown in the start output, with no `models.json` edit.
- Unknown or unreachable proxy: a clear warning and the `models.json` value, not a silent guess.
- Tests with a fake proxy response (loaded, not loaded, unreachable).

/goal Make lmcoder runs use the server's real context window per run, or stop and report when Pi offers no
per-run override and an owner decision is needed. Circuit breaker: on a loop or no progress, clear the goal and
write a failure report. Re-check Pi's version and the proxy's model list first; written 2026-10-29.
