# 714 — harnez usage shows Claude/GPT and Gemini unavailable after collectors are killed

**Status**: Open — live repro 2026-10-22: stale threshold equals the 5m collector interval
**Priority**: P2 (Medium)
**Severity**: Major
**Category**: Bug
**Related**: 710, [[527-agy-live-quota-fetch-is-killed-on-most-agent-turn-boundaries]],
[[649-harnez-usage-compact-no-longer-shows-agy-gemini-quota-rows]],
[[657-usage-compact-drops-claude-and-codex-writers-store-label-window-keys-cross-provider-observation-links-codex-limits-not-collected]]

---

/goal Restore current Claude/GPT and Gemini usage rows in `harnez usage`, with regression
coverage and live verification, or stop and report when blocked on user input or denied permission.

## 1. Problem & Motivation

User report: the `harnez usage` UI shows both AGY quota pools as unavailable:

```text
Claude/GPT unavailable (signal: killed)
Gemini unavailable (signal: killed)
```

Both usage rows are affected. The signal does not identify which process was
killed or whether a shared collector, timeout, or cancellation caused it. The
host, exact command, and observation time were not provided; collect them during
reproduction.

## 2. Findings

Related issues cover earlier AGY fetch kills (#527), missing Gemini rows (#649),
and usage-store projection failures (#657). This is a fresh observation; reproduce
it against current code and determine whether it shares one of those causes before
choosing a fix.

## 3. Implementation & Verification Plan

- Trace the failing collection path and identify who sends the kill signal and why.
- Fix the cause or expose a useful, provider-specific failure while retaining any
  valid cached usage.
- Add regression coverage for both pools; run `make test-q1` and verify the
  installed `harnez usage` UI when the required live services are available.

## 4. Recurrence (2026-10-22)

User saw it again in the live `harnez usage` view, now on three rows:

```text
Claude/GPT unavailable (signal: kill…
Gemini 4 Argon unavailable (signal: kill…
Gemini unavailable (signal: kill…
```

Since the original report, collection moved to `harnez server` with a 5-minute snapshot
cadence (828-830, 837, 839), so the killer may now be the server collector's timeout or socket
takeover (837) rather than a client. Check that path first. Rows also go gray often in the same
view; see 710.

## Live check (2026-10-22, lean sprint): not reproducible today

`harnez usage --raw` showed fresh Claude, Codex and both AGY pools. Gray comes from either a `QuotaFetchError`
(`internal/usage/types.go:160`) or snapshot age (`internal/usage/freshness.go:5`); the display does not say
which. Way forward: show the reason on a gray row (fetch error text or age), keep the last good quota on a
failed refresh, test fresh/stale/failed rendering. Then park until the next live occurrence, which the reason
text will explain.

## Live repro (2026-10-22 15:30, v0.2.0, user screenshot of `harnez usage -w`)

Claude Code and all AGY rows gray, Codex white. `harnez usage` at 15:30:34: Claude "5m ago", AGY "9m ago",
Codex "4m ago". No fetch errors: `--json` has no error field, AGY refreshed fine at 15:33:38.

Root cause (gray without a failure): `internal/usage/freshness.go` marks a snapshot stale at
`age >= DefaultCollectorInterval()` (5m), the same 5m cadence the server collects at. Every provider goes gray
for a moment before each refresh, and longer when a cycle runs late or is skipped. AGY at 9m means one missed
cycle: the server was restarted at 15:25:22 (v0.2.0 install) and the first cycle after the restart did not
refresh AGY.

Also seen: the gray row in `-w` showed no reason (the 714 reason text didn't appear in the cropped view;
`--json` carries no staleness field), and every cycle logs
`drain telemetry spool: ... CHECK constraint failed: score ... BETWEEN 1 AND 5 (275)`, so one bad spool
entry fails each drain.

Way forward: stale only after a grace beyond the cadence (e.g. 2 intervals, value in `spec/usage.yaml`);
collect all providers in the first cycle after a server start; expose `stale`/`reason` in `--json`;
skip or quarantine the invalid spool entry instead of failing the drain. Test that a snapshot exactly one
interval old renders normal, not gray.

Statusline feed (checked 15:45): both statuslines are wired (`harnez statusline`, `harnez statusline --agent agy`
in `~/.gemini/antigravity-cli/settings.json`) and post to `/usage/passive`. Live: one Claude statusline render
moved the Claude row from "1m ago" to "just now". Yet AGY stayed 9m old with an AGY session open, so either the
AGY payload carries no quota fields (`passiveWindows` then drops it silently) or AGY renders only on activity.
Check: log or count dropped statusline observations per provider; capture one real AGY payload.
