# 1214 — agent list token totals: codex terra overcounted ~3x vs codex state DB, loom runs recorded as 0 tokens

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Major
**Category**: Bug
**Related**: 1213, 1211, 985

---

## Problem

2026-11-10, Codex sessions since 09:57Z:
- `harnez agent list --archived --dead --json`: harnez repo terra:low 28 runs, `input_tokens_total` sum 62.8M
  (cached 60.9M), luna 2.7M; loom repo 8 terra + 1 sol runs, all 0 tokens.
- Codex itself (`~/.codex/state_5.sqlite` `threads.tokens_used`, 10 threads updated since then): 19.4M total;
  rollout logs show one gpt-6.1-sol thread with 1.67M input.

Unverified cause: cumulative `total_token_usage` summed per turn/resume instead of taking the last value;
loom sessions maybe never parsed (different dir or harness).

## Goal

/goal harnez per-session Codex token totals match Codex's own thread totals (state DB / last `token_count`
event) within 1%, including sessions in other repos. A test with a fixture rollout of several turns and a
resume pins it. On a loop or no progress, clear the goal and write a failure report.
