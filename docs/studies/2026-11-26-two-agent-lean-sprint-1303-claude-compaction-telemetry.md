---
kind: sprint
date: 2026-11-26
ticket: 1303
branch: sprint/1303
models:
  searcher: skipped # plan-only developer turn inspected the live code
  developer: claude:haiku:med (M1, replaced), then claude:haiku:high
  reviewer: agy:neon:med # one-shot over M1 (M1-M1e); the host reviewed every milestone live
---

# Two-Agent Lean Sprint: #1303, Claude compaction telemetry

## Outcome

Each Claude compaction now records:
- **Tokens and model** on the PreCompact/PostCompact `compaction_events` rows. They come from the transcript's `compact_boundary` line.
- **A priced economics row.** Input tiers are applied, so a compaction above 100k tokens is priced at the 5x Haiku rate.
- **One quota sample** in `turn_quota_boundaries`, boundary `compact`. It is cache-first, with a non-blocking cross-process lock, and runs in the collector.
- **Redacted payloads:** `compact_summary` and `custom_instructions` are removed from the hook payload before it is posted or spooled.

The PostCompact hook stays at about 110 ms.

Final live check: "102474 -> 14061 tokens".
- Row 863 has 14061 tokens and model `claude-haiku-5-5`, filled at `retry:1`.
- Economics: 51237 µ$ = 102474 × 500000 µ$/M.
- Quota sample 2537, `compact-fetch`, fresh.

## Key finding

Claude Code stamps the compact boundary before PostCompact but appends it to the transcript only after the hook returns.
- Live evidence: the boundary is stamped 15:03:44.488Z, and the file mtime is 15:03:49.081Z, after a 3.68 s in-hook wait ended.
- In-hook polling (M1b-M1d) could never see it. The fix is a deferred fill in the long-running collector: re-reads on the `claude_compact.fill_retry` schedule, plus a fill on the session's next event after a restart.

## Milestones

| Step | Commit | Result |
|---|---|---|
| M1 read boundary at PostCompact | ed6c0b44 | fixtures pass, live rows empty |
| M1b in-hook poll | 4b7a6815 | live empty |
| M1c diagnostic note | c3bdc52b | showed the old boundary was taken |
| M1d wait for this compaction's boundary | ce7279ff | live empty, timing proved the write order |
| M1e deferred fill in the collector | fbe30687 | live filled at retry:1 |
| M2 tiers, atomic fill, quota sample | 6345ca28, f833d576 | live: tier price, sample stored |
| M3 neon review fixes | 35926554 | live filled, hook 109 ms |

## Lessons

- **Canary first.** The plan review dropped the M1 code canary. Four live failures followed before the write order was measured. A one-minute manual compact with timestamps would have shown it at the start (`docs/Canary.md`).
- **Diagnostic note early.** The M1c reason/timestamp note turned blind retries into one decisive measurement.
- **haiku:med vs haiku:high.**
  - haiku:med passed fixtures but failed live, and ran the test suite three times against a one-run budget.
  - haiku:high kept the budget on every milestone and delivered M1e-M3 first try against the pre-work.
  - Use haiku:high when the work crosses process or timing boundaries.
- **neon:med one-shot review (3m10s)** found two HIGH issues that the host's live checks had passed: summary text in the spooled payload, and timers that outlive shutdown. Both were fixed in M3.
- **A scratch HOME does not isolate hook telemetry.** The canary hook calls reached the live collector through the broker fd or socket. Test rows in the real DB await the owner's decision.

## Open

- A timer armed after `StopClaudeFills` runs until process exit, because there is no closed flag. This is LOW.
- `max_transcript_bytes` (8 MiB) is not measured.
- The #1302 report does not consume the compact samples yet (`docs/Telemetry.md`).
