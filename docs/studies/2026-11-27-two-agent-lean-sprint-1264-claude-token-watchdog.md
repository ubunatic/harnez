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

## Test report

### Canary runs

Same read-only Haiku prompt in every run: read five files (10,729 lines) in 500-line ranges.
Numbers come from the canary transcripts' usage fields only.

| Run | Binary | Claude window | Peak | >100k | Outcome |
|---|---|---|---|---|---|
| 1264b stream | before | 200k | 169k | 3 | ran to end; watchdog never fired; Claude compacted at 166k and 169k |
| 1264b json | before | 200k | 170k | 3 | ran to end; watchdog not armed; 191,341 µ$; end `context_tokens` 65k hid the 170k peak |
| 1264c stream | 284967aa | 100k | 81.2k | 0 | completed; Claude compacted 3x at 96-106k; `[done:]` shows peak 81.2k |
| 1264c json | 284967aa | 100k | 84.8k | 0 | completed; `peak_context_tokens` 84,801, `context_tokens` 43,496 |
| 1264d stream | 284967aa | 200k (env) | 134.9k | 1 | stopped; status `stopped`, exit 1, resumable |
| 1264d json | 284967aa | 200k (env) | 134.8k | 1 | stopped; one JSON document; turn record 3 calls, 236,047 cached, 123,360 cache write |
| 1264d resume | 284967aa | 200k (env) | 23.3k | 0 | `/compact` first at 134.9k; dangling `tool_use` after the kill was no problem |

Between the before and after runs the Haiku `autoCompactWindow` in the owner's Claude settings
changed from 200k to 100k, so 1264c tested Claude's compaction, not the watchdog. 1264d set
`CLAUDE_CODE_AUTO_COMPACT_WINDOW=200000` on the canary process only.

### Tests added or changed

| Step | Commit | Tests | What they prove |
|---|---|---|---|
| M1 | 5876f571 | `agent_claude_watchdog_test.go`, 6 cases | A streaming Claude turn stops when one request passes the limit; counts are per request and main chain only; fixtures follow one live `claude -p --output-format stream-json` capture (claude-code 2.1.330). Both stop tests fail without the fix. |
| M2 | dabe6218 | `agent_claude_meter_test.go`; `TestParseClaudePartialCountsCompleteCallsOnly`; repeated-id case 3x40k; quota-lifecycle stub | A stopped turn writes a turn record, totals and quota readings; a truncated last line is skipped; one message id streamed three times at 40k does not stop. |
| M3 | 284967aa | `TestDetachedStartWorkerKeepsWatchdogStop`, `TestDetachedResumeWorkerKeepsWatchdogStop` | Detached workers keep status `stopped` and the stored count; before the fix they overwrote it with `failed`. |
| M3 | 284967aa | `TestClaudeWatchdogJSONStartWithoutProviderIDStops`, `TestClaudeWatchdogJSONResumeStopsWithOneDocument` | Every `--json` stop path prints exactly one document and exits non-zero. |
| M3 | 284967aa | `TestClaudeResumeStopStoresObservedProviderSessionID` | A stale-resume fallback that is then stopped stores the new provider id. |
| M3 | 284967aa | `assertStoppedTurnTotals`; quota reads == 2 | Session totals and tokens-since-compact after a stop; exact quota read count. |
| M3 | 284967aa | `TestParseClaudePartialPeakIsMainChainMax`, `...ExcludesSubagentFromPeak`, `TestParseClaudeCompletedTurnPeak`, `...RepeatedIDCountsOnceAtLastUsage` | Peak 90k above last call 60k (the canary shape); sub-agent prompts never set the peak; completed turns report the same peak; a repeated id counts once at its last usage. |

### Full test-suite runs

| Run | By | Result |
|---|---|---|
| M1 `make test-q1` | developer | 83 ok, 0 FAIL; host package rerun passed |
| M2 `make test-q1` | developer | 1 FAIL (quota-lifecycle stub); fixed, touched packages green |
| M2 `make test-q1` | host | 83 ok, 0 FAIL |
| M3 `make test-q1` | developer | 83 ok, 0 FAIL; host package rerun passed |

### Review findings

| Sev | Finding | Source | Handled |
|---|---|---|---|
| HIGH | Detached start and resume workers overwrote `stopped` with `failed` | host diff review, M2 | M3: typed `watchdogStopError`, 2 tests |
| MED | A stopped turn was missing from cost and quota telemetry | host, M1 | M2: partial parse and metering |
| MED | `--json` start stop with empty provider id printed nothing and skipped quota | host, M2 | M3, tested |
| MED | Weak meter assertions; no fixture with peak above the last call | host, M2 | M3, 5 tests |
| LOW | Repeated-id case could not fail; resume-after-stop untested; stale-resume id; resume JSON stop untested | host and reviewers, M1-M2 | M2 and M3 |
| OK | Counts per request, main chain; resume compacts first; metering counts once; `claudeStopError` keeps Claude's errors | opus:med reviews, M1 (1m55s) and M2 (1m19s) | confirmed |

## Lessons

- **Canary first paid off twice:** the before-canary showed the stream path was broken too (not only `--json`), and the rerun exposed the owner's new Claude window that hid the stop.
- **A canary must pin the environment it depends on.** The 1264c rerun silently tested Claude's compaction, not harnez's. Set the native window explicitly on the process.
- **Host diff review found the HIGH** (detached worker overwrote `stopped` with `failed`) that the M2 reviewer missed; the reviewer's value was confirming metering counts once.
- **haiku:high** kept the one-run test budget on all milestones.

## Open

- LOW: a stopped turn's `output_tokens` comes from provisional stream usage (10 for 3 calls).
- Owner option: set `CLAUDE_CODE_AUTO_COMPACT_WINDOW` on the Claude child from the harnez threshold, so Claude compacts before the 5x band.
