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

## Audit 2026-11-11

Jules audit (PR GH#162, report `reports/jules/1214.md`, checked against 33dacc94, 6 of 7 references
held; `internal/usage/codex.go:137` is really line 156). Verdict "partly done"; read as still valid, with leads:

- `harnez usage` project scan sums every rollout file: `internal/usage/project.go:317-327` walks
  `rollout-*.jsonl` and adds each file's breakdown, while `internal/usage/codex.go:156` keys the latest
  value per session ID. If a resume writes a new rollout file carrying prior totals, this overcounts.
  Unverified: whether Codex resumes write a new file with cumulative totals.
- `agent list` totals come from `internal/subagent/codex.go`: `TokensTurn` (line 491) sums only
  `turn.completed` usage (line 465); `token_count` events set only `ContextTokens` (line 476).
  A run with no `turn.completed` events records 0 tokens, a plausible cause of the loom zeros
  (unverified: needs a loom run's captured output). The fixture `codex-987-rollout.jsonl` has 3
  `token_count` and 0 `turn.completed` events.
- The ~3x overcount on `agent list` is not explained by the report; next step is to compare one
  terra session's recorded `input_tokens_total` with its rollout's last `total_token_usage`.

## Jules runs

| kind | session name | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| audit | jules-audit-1214 | 10577886768464323199 | github #162 (closed) | done: partly done, 6/7 refs hold |
