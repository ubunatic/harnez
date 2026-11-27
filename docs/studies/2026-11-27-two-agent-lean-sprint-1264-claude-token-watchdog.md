---
kind: sprint
date: 2026-11-27
ticket: 1264
branch: main
models:
  searcher: claude:haiku:low
  developer: claude:haiku:high (dev-1264, M1-M3 and docs)
  reviewer: claude:opus:med # one-shot per milestone (M1, M2)
---

# Two-Agent Lean Sprint: #1264, token watchdog on every Claude turn

## Outcome

The harnez token watchdog (592) now arms on every Claude turn: streaming, `--json` and detached.
At the first request over the threshold it kills the process group, meters the finished calls,
stores the count so the next resume compacts first, and reports `peak_context_tokens`.

| Canary (Haiku, threshold 100k, same prompt) | Peak | Requests over 100k | Result |
|---|---|---|---|
| 1264b before, stream / json | 169k / 170k | 3 / 3 | ran to completion, 191341 µ$ (json) |
| 1264d after, stream / json | 134.9k / 134.8k | 1 / 1 | `stopped`, exit 1, json turn metered 72149 µ$ |
| 1264d resume | 23.3k | 0 | `/compact` first, reply in 6 s |

## Key findings

- **Root cause:** `DriverFor` returned the batch Claude driver; only a tool-call limit upgraded it to the stream driver, so `Observe` never saw a Claude event. The `--json` gate was a second, smaller cause.
- **The watchdog cannot prevent the crossing request.** One tool result jumped the prompt 80k -> 135k. It cuts the 5x requests from 3 to 1.
- **Claude's own window prevents it.** With the owner's Haiku `autoCompactWindow` 100000 (set 02:45 this night), Claude compacted at 96-106k and no request passed 100k (canaries 1264c: peaks 81.2k, 84.8k). Testing the stop needed `CLAUDE_CODE_AUTO_COMPACT_WINDOW=200000` on the canary process.
- **Resume after a kill with a dangling `tool_use` works**, so no SIGINT grace period is needed.

## Milestones

| Step | Commit | Result |
|---|---|---|
| Canary before | 7d5defac | watchdog never fired, 3 requests over 100k |
| M1 arm on streaming Claude | 5876f571 | 6 fixture tests, q1 83 ok |
| M2 meter stops, `--json`, peak | dabe6218 | host q1 83 ok; review: 1 HIGH found by host (detached status) |
| M3 review fixes | 284967aa | 11 new tests, q1 83 ok |
| M3 docs | 3a6130ac | AutoCompaction, Architecture §4.1 |
| Acceptance canary | 657d9974 | stops at 134.9k / 134.8k, resume compacts |

## Lessons

- **Canary first paid off twice:** the before-canary showed the stream path was broken too (not only `--json`), and the rerun exposed the owner's new Claude window that hid the stop.
- **A canary must pin the environment it depends on.** The 1264c rerun silently tested Claude's compaction, not harnez's. Set the native window explicitly on the process.
- **Host diff review found the HIGH** (detached worker overwrote `stopped` with `failed`) that the M2 reviewer missed; the reviewer's value was confirming metering counts once.
- **haiku:high** kept the one-run test budget on all milestones.

## Open

- LOW: a stopped turn's `output_tokens` comes from provisional stream usage (10 for 3 calls).
- Owner option: set `CLAUDE_CODE_AUTO_COMPACT_WINDOW` on the Claude child from the harnez threshold, so Claude compacts before the 5x band.
