# 714 — harnez usage shows Claude/GPT and Gemini unavailable after collectors are killed

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Major
**Category**: Bug
**Related**: [[527-agy-live-quota-fetch-is-killed-on-most-agent-turn-boundaries]],
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
