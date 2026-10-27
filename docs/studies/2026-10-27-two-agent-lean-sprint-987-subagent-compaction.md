---
kind: sprint
---

# Two-agent lean sprint: 987, subagent compaction for every provider

Date: 2026-10-27. Host: Claude Opus 5.5. Architect `agy:argon:low` (`argon-987`). Developer
`codex:gpt-6.1-terra:med`, one fresh session per milestone (`dev-987-m1` … `dev-987-m4b`), each with a plan-only
turn, then a write turn. Cost question answered by the owner's native Claude session `searcher`.

## Course

- M1 measured before building: which hooks and stream events each provider emits around a compaction.
- The owner questioned lowering the Codex threshold from 200k to 100k. The searcher priced one Codex compaction at
  about $0.019, paid back in about 0.64 turns at 10 tool calls per turn; the owner kept 100k.
- M4 was planned above ~400 lines and split into M4a (state, pairing) and M4b (Claude hooks, agy fallback).

## Delivery

- M1 `f726353c`: probes in `docs/AutoCompaction.md`. Codex `--json` has no compaction event and cumulative usage;
  the rollout `compacted` item and Pre/PostCompact hooks are the signal. Claude PostCompact fires in `-p`. agy has
  no hooks; its stream shows a `checkpoint` step.
- M2 `6697c28a`: per-model Codex thresholds (terra/sol/luna 100000), on-demand limit 50000 and status prompt in
  `spec/agent.yaml`; COMPACT column in `agent models`.
- M3 `de8f8995`: `CodexDriver.Compact` (resume with `-c model_auto_compact_token_limit=50000`, confirmed from the
  rollout). Live: 71705 -> 17063.
- M4a `2cce6a88`: stored context and count after compaction; `LatestCompactionPair`; `compaction_pair` in
  `agent status --json`.
- M4b `579d7e0c`: Claude PreCompact/PostCompact hooks via `harnez apply` (spec file, idempotent); agy
  `stream_fallback` pair. Live: Claude 46570 -> 3718 (source hook), agy pair (source stream_fallback).

## Findings

1. The developer committed after a failed `make test-q1` twice (M4a, M4b) and reran only touched packages. The host
   full run (issue 896 rule in the lean-sprint skill) caught nothing new both times; the rule held.
2. A fresh developer per milestone kept cached re-read at about 1.8–6.1M tokens per milestone, against the growing
   context of one long developer session.
3. A new telemetry record kind did not land until the running collector was restarted: green tests, `null` pair
   live, no message. Filed as 989.
4. Live context checks on Codex need real context: Codex truncates long command output, so a session stayed at
   18–35k until three chunked reads reached 71.7k.
5. Host query errors: several telemetry checks hit `usage.db` instead of `telemetry.sqlite`; `agent list` hides
   completed sessions without `--dead`.

## Not verified live

A killed compaction (Pre without Post) and Codex native pre-prompt compaction at 100k; both unit-tested.

## Follow-ups

- 989: stale collector after `make install`.
