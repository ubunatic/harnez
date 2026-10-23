# 875 — warn the orchestrator when its shells, agents or the host run short on memory, swap or CPU

**Status**: Open
**Priority**: P2 (Medium) — resource trouble surfaced only when the user noticed it
**Severity**: Minor
**Category**: Feature
**Related**: 870 (open-shell brief after each new shell), 873 (usage never hangs), 874 (slow store read)

---

/goal The orchestrator gets one short warning line when a shell or agent it started (including that agent's
child processes) uses a lot of CPU or memory, or when host swap or the RAM-backed `/tmp` is nearly full, so it
can tell the user early; thresholds in spec, verified by tests per warning type and one live Claude Code
session; or stop and report when a measure cannot be read reliably.

## Motivation (2026-10-23)

The user noticed swap at 99% and heavy CPU from another session before any agent did. Findings: swap is zram
(compressed RAM, 8 GB) and stayed full; about 4.5 GB of it was idle files in `/tmp` (tmpfs, 4.7 GB used: old
playwright browsers, a leftover Go test dir `TestRunCrispasrGPU...`, staging tars, Go caches), the rest
day-old lmcoder llama-servers and idle desktop services. No OOM kills that day. Load average 4-5 from another
session's test runs, which also made `harnez usage --offline` take 8s (874).

## Behaviour

- Per shell/agent: harnez knows the PIDs of agents it starts; sum CPU and RSS+swap over the process tree.
  Warn above spec thresholds, e.g. `dev-710 (codex): 3.1 GB, 180% CPU for 5m`.
- Host: warn when swap or `/tmp` passes a spec threshold, naming the top owners it can see, e.g.
  `swap 97%: /tmp 4.5 GB, llama-server 0.8 GB`.
- Delivery: in the 870 brief after a new shell, and at most once per condition per interval (spec), not on
  every tool call. Never guess: unreadable values are left out.

## Open questions (record answers, do not invent)

1. Which hook delivers it on hosts other than Claude Code (same question as 870).
2. Whether the harnez server should sample in the background (cheap, already running) or the hook should
   sample on demand.
