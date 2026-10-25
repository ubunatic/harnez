# 952 — usage rows gray: leftover expired or probe windows mark whole agent stale

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 714 (staleness grace), 710 (AGY gray rows), 107 (dimming)

---

/goal `harnez usage` dims only rows whose own data is stale: an old leftover window no longer grays a whole agent
whose other windows were just refreshed, and probe/test windows never land in the real usage store. A
reproduction test fails first, then passes; the owner sees AGY rows white on a live run. Or stop and report on a
design choice. Circuit breaker: on a loop or no progress, clear the goal and write a failure report here.

## Evidence (live, 2026-10-25 ~19:02 UTC)

Screenshot: All Usage box, Claude Code, Claude/GPT, Gemini 4 Argon and Gemini rows gray; only OpenAI Codex white.
`harnez usage --json`:

- AGY: `last_refreshed` 19:00:45 (fresh), but `stale: true`, `stale_reason: "window expired: default (stale)"`.
  Its `session` window is `{name: "default (stale)", source: statusline, reset_at: 2026-10-10T16:00:00Z}`: a
  15-day-old statusline window that is still carried. `StaleReasonText` (`internal/usage/types.go:194-224`)
  returns stale for the whole agent as soon as any window has expired, so every AGY model-group row (fresh
  data) is dimmed.
- Claude Code: `session.name` is `"M2 rework live probe (stale)"`, `source: probe`. A probe/test window name was
  written into the real store and survives; it shows as a 5h row that disagrees with the registry `five_hour`
  window (0%). At screenshot time Claude was also gray; at 19:02 it reported `stale: false`.

## Why

1. Agent-level staleness is the OR of all windows, including windows that are not shown and whose data is old
   leftovers; an expired window should be dropped or dimmed alone, not dim its siblings.
2. Nothing removes a session window whose `reset_at` passed long ago, nor a window from a non-production source
   (`probe`).

## Store cleanup

- Searched repository tests and scripts for the `M2 rework live probe` key and `source: probe`; no pre-existing emitter was found. The passive update/store path accepts source-tagged windows, and its integration test uses temporary HOME/XDG directories.
- Before cleanup, backed up `/home/uwe/.local/share/harnez/telemetry.sqlite` to `/home/uwe/.local/share/harnez/telemetry.sqlite.issue-952-20261025.bak` (174182400 bytes; backup confirmed to contain 18 matches).
- In one SQLite transaction, deleted 18 Claude `probe` observations whose window key was `m2_rework_live_probe`; each observation contained only that target window. Post-delete match count: 0.
- After install, `harnez usage --json` reported Claude and AGY `stale: false`; Claude showed current API windows and AGY showed its model groups without the expired default session. The probe match count remained 0.
- `make test-q1` passed.

## Plan

1. Repro tests: an AGY usage with fresh model groups plus an expired `default` session window must not be stale
   as a whole; a probe-sourced window must not be persisted to the real store.
2. Fix: compute staleness per window/row in the render (keep the agent-level `stale` for the snapshot age and
   fetch errors only); drop or replace a session window whose `reset_at` is older than its period, and the
   statusline `default` window once model groups exist.
3. Find where `source: probe` windows are written (likely a live-probe test or canary writing through the real
   store path) and isolate it to a temp store; purge the existing probe row from the store once.
4. Verify live: `harnez usage` and the watch TUI with a pty, AGY and Claude rows white while fresh.
