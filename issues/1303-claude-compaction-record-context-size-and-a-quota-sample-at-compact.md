# 1303 — Claude compaction: record context size and a quota sample at compact

**Status**: Closed — Live compaction 102474->14061 on a Haiku worker: PostCompact row 863 filled with tokens and model at retry:1 by the collector's deferred fill (Claude writes the boundary after PostCompact returns), economics 51237 µ$ at the >100k tier rate, quota sample 2537 (compact-fetch, fresh), hook 109ms; summary text redacted before post/spool; q1 83 ok 0 FAIL. Open LOW: timer armed after StopClaudeFills, 8MiB read bound unmeasured, #1302 report does not yet consume compact samples.
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: #1302 (Claude cost report, the consumer), #423 (compaction token economics), #987 (subagent compaction), #1301 (quota pairing)

---

## Goal

/goal Make each Claude compaction useful for cost analysis:
- the compaction rows carry the context size and the model;
- the compaction cost calculation works for Claude;
- each compaction stores one quota sample, so the #1302 report can pair the API cost between two compactions with the quota rise.

Verify on a live compaction. Stop and report when the hook payload or the transcript lacks the data, or a design choice is the owner's. On a loop or no progress, clear the goal and write a failure report.

## 1. Problem & Motivation

Owner, 2026-11-26: "Compact happens not too often, it may be a good point to actively fetch usage data."

Today a Claude compaction records almost nothing:
- The PreCompact and PostCompact hooks run `harnez codex-telemetry`, and `internal/server/codex_telemetry.go:77` writes `compaction_events` rows for them.
- For Claude, every token field and `model` is empty. All 61 rows on 2026-11-26 are like that, including this host session's manual compact at 12:13:04 (trigger `manual`, source `hook`).
- So `ReconcileCompactionEconomics` has nothing to price for Claude.
- Interactive host sessions have no per-turn quota rows. The #1302 report shows "n/a" or relies on dense quota-history polling for them.
- Compactions are about an hour of work apart, long enough to rise several 1% steps of the weekly limit. Single turns do not (#1301).

## 2. Findings

- The handler was written for Codex payloads, which carry tokens (`codex_telemetry.go:77-95`).
- Harnez already reads the compact context size from Claude transcripts for worker sessions: `internal/subagent/claude.go:202-284` (`e.Compact.PreTokens` / `PostTokens`).
- Stale comment: `internal/compactcheck/compactcheck.go:6-7` says Claude Code has no PostCompact hook. It has one, configured in `~/.claude/settings.json`, and it ran at this session's compact.
- Unverified: the exact fields of Claude's PreCompact and PostCompact hook payloads. Expected are `session_id`, `transcript_path` and `trigger`. The first step is a canary: a manual compact on a Haiku worker, logging the payload keys only.
- Risk: the usage endpoint returned 429 in #1301. The sample must use the existing cached or locked fetch path and record `captured_at`/cache age. It must never block or fail the compact; on timeout it writes no sample.

## 3. Plan

- **M1 canary.** Log the PreCompact and PostCompact payload keys of one Claude compact. Confirm that the transcript's compact boundary carries the pre-compaction and post-compaction token counts.
- **M2 fill rows.** Fill the Claude `compaction_events` tokens and model from the transcript, so compaction economics are computed for Claude. Fix the stale comment.
- **M3 quota sample.** Store one quota sample at each compaction, with a time bound and through the cache path, as a boundary that #1302 can pair. Put any timeout in `spec/` with a schema entry.
- **Verify** on a live compaction:
  - the row has tokens and model;
  - the economics row is filled;
  - the quota sample is stored with its cache age;
  - the hook's run time stays within the spec bound.
- **Reload note:** hook command changes reach other Claude sessions only after `harnez apply` and a session reload. Tell the owner when that point comes.

## Decision (decide mode, 2026-11-26)

- **Order.** Runs after #1304, so the compaction work goes into the single hook dispatcher, not into a new settings entry.
- **Developer.** `claude:haiku:med`, starting with a plan-only first turn. M1 is a canary and M2 fills rows from data harnez already parses (`internal/subagent/claude.go:202`).
- **Escalate to `claude:haiku:high`** for M3 (the quota sample), if the plan shows the fetch path needs lock or timeout work.
- **Review.** A one-shot `agy:neon:med` reviewer after M3, covering the whole range.

## Plan turn review (host)

Developer plan (dev-1303, haiku:med, read-only):
- The row write is at `internal/server/codex_telemetry.go:77`.
- The Claude transcript parser is at `internal/subagent/claude.go:189-190,246-248`.
- The quota path is `CaptureTurnQuotaSinceCache` (`internal/usage/turnquota.go:60-106`). Its Go constant `TurnQuotaTimeout = 1800ms` (`:31`) breaks the spec rule.
- The live-fetch lock is in-process only (`internal/usage/claude.go:370`).
- The planned storage is `turn_quota_boundaries` (`internal/usagestore/store.go:680`).

The host checked the plan's open facts against primary sources instead of running a code canary:
- **Hook payload** (https://code.claude.com/docs/en/hooks):
  - PreCompact gets `session_id`, `transcript_path`, `cwd`, `hook_event_name`, `trigger` and `custom_instructions`.
  - PostCompact gets the same common fields plus `trigger` and `compact_summary`.
  - No `turn_id`, `model` or token counts. Only SessionStart carries `model`.
- **Transcript boundary:** the host session's own transcript, read as key names and numbers only, has `type: system`, `subtype: compact_boundary` and `compactMetadata` with `trigger`, `preTokens` (166948), `postTokens` (27143), `durationMs` and `cumulativeDroppedTokens`.
- **Claude SessionEnd:** not in the Claude manifest (`spec/hooks.yaml`), so no Claude SessionEnd reaches `codex-telemetry`.

## Decision (decide mode, plan review)

- **Milestones:** drop the M1 code canary, since the shapes above are verified. The new plan:
  - M1 fills the Claude rows (dev-1303, haiku:med).
  - M2 stores the quota sample. It goes to a fresh developer on `claude:haiku:high`, per the ticket's escalation rule, because it needs lock and timeout work. A fresh session also avoids the context growth seen in #1304 M4.
- **Tokens:** come from the transcript's `compact_boundary` `compactMetadata`, read at PostCompact.
  - PostCompact fills the pre-compaction and post-compaction tokens for that compaction: the newest boundary whose timestamp is at or after the PreCompact row's time.
  - PreCompact keeps writing its row; its tokens may stay empty.
  - Model: the main-chain model before the boundary.
- **Privacy:** never store, log or print `compact_summary` or `custom_instructions`. A test proves that neither reaches any DB column.
- **No reload needed:** the hook commands do not change. A new binary (`make install`) takes effect on the next hook call in every session.
- **Lock:** the compaction sample is cache-first.
  - It does a live fetch only if the cache is older than a spec age.
  - The fetch runs under a cross-process, non-blocking file lock. If the lock is held, it records the cached sample with its age and does not fetch.
  - Reason: the 429 in #1301, and several Claude sessions can compact at once.

## M1 pre-work (fill Claude compaction rows)

1. Use fixture transcripts in the verified shape: assistant usage lines with `message.model` and `message.usage`, then a `system`/`compact_boundary` line with `compactMetadata.preTokens`/`postTokens`/`trigger`. Use a temp HOME; never read real transcripts.
2. PostCompact: fill `compaction_events` tokens and model, and the `token_snapshots` precompact/postcompact rows `ReconcileCompactionEconomics` needs. A test asserts the economics row for a Claude compaction is filled (not skipped) with the fixture's numbers.
3. Negative cases:
   - no boundary yet;
   - only an older boundary from an earlier compaction;
   - missing `transcript_path`;
   - an unreadable file.

   Each one writes the row with its tokens empty, exits 0 and adds nothing to stdout.
4. Privacy test: a payload with `compact_summary`/`custom_instructions` sentinels; no DB column or stdout holds them.
5. Codex rows must stay byte-identical; prove it with the existing Codex tests unchanged.
6. Fix the stale "no PostCompact hook" comments at `internal/compactcheck/compactcheck.go:6-7` and `cmd/harnez/compactcheck.go:3-4,28`.
7. Gates: run `HTO=0 make test-q1` once, logging to a file. Commit as `feat(compact): … (issue 1303 M1)`. Never amend.

## M1 review (host)

M1 delivered (Claude rows from the transcript boundary): ed6c0b44. q1: 83 ok and 0 FAIL, but on the third run; the budget is one run per milestone. The two failed logs are `q1-1303-m1-run{1,2}-fail.log` in the host scratchpad.

Host diff review: the reader keeps only trigger, token counts and model, and drops `<synthetic>` models. Read errors count as no boundary. The PostCompact row is filled only if the boundary is not older than the session's newest PreCompact row. These are fine.

The live check fails: the rows stay empty on a real Claude compaction.
- The host ran `harnez agent compact --name dev-1303` twice; the second run came after restarting the collector service on the M1 binary.
- Session `e3521bad-8c46-463a-a838-d48ba049f618`, rows 794/795:

  | Row | Time (UTC) | Tokens and model |
  |---|---|---|
  | precompact 794 | 14:24:50.678 | empty |
  | postcompact 795 | 14:25:12.400 | empty |

- The transcript's boundary is at 14:25:12.311Z with `compactMetadata.preTokens` 12948. That is after the PreCompact row and before PostCompact ran, so `applyClaudeCompact` should have applied it.
- The binaries match: `~/go/bin/harnez` links to `~/.local/bin/harnez` (built 15:21), and the systemd unit runs the same file.
- Cause unknown. The fixture tests do not run the real path: Claude → `harnez hook PostCompact` → dispatcher `codex-telemetry` handler → collector service → `ProcessCodexTelemetryTx`.

Side finding:
- A scratch `HOME` does not isolate hook telemetry. `HOME=<tmp> harnez hook PreCompact/PostCompact` wrote to the real `~/.local/share/harnez/telemetry.sqlite` through the running collector socket.
- Polluted rows: 789-791, session `s-canary`, from a host check. Deleting them waits for the owner.
- The search for the `compact_summary` sentinel in the DB found nothing.

## M1b pre-work (make the live path work)

1. Reproduce the live failure with a test that runs the real chain end to end:
   - It sends a Claude PostCompact payload through `harnez hook PostCompact` (dispatcher → codex-telemetry handler → the same IPC/server or direct write the hook uses in production).
   - It uses a fixture transcript whose boundary follows a PreCompact row.
   - It fails on ed6c0b44. Find the cause, using the session above and the verified boundary shape. Candidates:
     - the hook client vs server split around `PrepareCodexTelemetry`/`harnez_context`;
     - JSON field names in `codexEvidence.Compact`;
     - time comparison or parsing.
2. Fix it. The test must then pass, with no assertion weakened.
3. The hook path must honour an isolated HOME/data dir, or document why it does not. A test proves that the M1b tests never write to the real data dir. Simplest is an env override that tests set.
4. Live check, run by the host after the commit. Leave the session in a state where `harnez agent compact` can run.
5. Gates: one `HTO=0 make test-q1`, logging to `q1-1303-m1b.log`, one run only. If that run fails, fix it, rerun only the touched packages, and report that the full run is owed. Commit as `fix(compact): … (issue 1303 M1b)`. Never amend.

## M1b review (host)

M1b delivered (PostCompact waits for the boundary flush, bounds in `spec/hooks.yaml` `claude_compact`): 4b7a6815. q1: one run, 83 ok and 0 FAIL. Red check on ed6c0b44: 3 tests failed as intended.

The live check still fails:
- `harnez agent compact --name dev-1303b`, session `eeaf12bb-31b0-4828-929c-c4b575ac775e`.
- The collector had just restarted on 4b7a6815; it had been up 1m46s.
- Rows 806/807 are empty.

The late-flush theory is falsified:
- The transcript boundary is at 14:36:54.031Z with preTokens 80748 and postTokens 16136.
- `harnez log` shows `hook PostCompact` exit 0 in **45 ms**. The PostCompact row was created at 14:36:54.375Z, after the boundary.
- So the hook neither waited for the 3 s bound nor applied a boundary that was already on disk.
- The developer's dispatch-chain test passes on ed6c0b44 too, so the test payload does not match the real one.

The remaining candidates sit before `readClaudeCompact` is called:
- the real payload lacks `transcript_path`, or has no `compact_summary`, so the wait gate is off;
- `e.Kind` is not `postcompact` on the hook side;
- the hook side never runs `PrepareCodexTelemetry` (broker FD path);
- `applyClaudeCompact` returns false (`LatestCompactionEventTime` / timestamp).

Second isolation incident: a test sent one `isolation-check` record to the live collector through `HARNEZ_BROKER_FD`. Deleting it waits for the owner.

Lesson: skipping the M1 canary cost two rounds. The documented payload and the fixture shape did not prove what the live hook sees.

## M1c pre-work (canary first, then fix)

1. Add a permanent diagnostic to the Claude PostCompact path: one `harnez log`/stderr-free telemetry note per compact.
   - It records the payload key names only (never values), whether `transcript_path` was set, the `e.Kind`, which process ran `PrepareCodexTelemetry`, and one outcome reason: `no_transcript`, `no_boundary`, `stale_boundary`, `no_precompact`, `applied` or `read_error`.
   - The host must be able to read it with one command; name that command in the report.
   - Test each reason. Never record `compact_summary` or `custom_instructions` values; extend the privacy test to cover the note.
2. Commit the diagnostic first as `feat(compact): … (issue 1303 M1c)`, then stop and report. The host runs a live compact and reads the reason. No fix this turn unless a test with a real-shaped payload already proves the cause.
3. The tests must not reach the live collector: clear `HARNEZ_BROKER_FD` and any socket env in every new test. Assert it.
4. Gates: one `HTO=0 make test-q1`, logging to `q1-1303-m1c.log`. Never amend.

## M1c review (host): diagnostic result

M1c delivered (diagnostic note per PostCompact): c3bdc52b. q1: 83 ok, 0 FAIL.

Live run: `harnez agent compact --name dev-1303b`, session `eeaf12bb-…`, 14:44:43Z PreCompact (83 ms), 14:45:18Z PostCompact (68 ms). Rows 812/813 are still empty. The note says:

```
"reason":"stale_boundary","hook":{"side":"hook","transcript_set":true,"keys":["compact_summary","cwd","hook_event_name","prompt_id","session_id","transcript_path","trigger"]}
```

The hook side found a fresh boundary:
- The transcript's newest boundary is at 14:45:18.352Z (pre 157210, post 11106, not a sidechain).
- PostCompact took 68 ms, so the 3 s poll did not run. `waitClaudeCompact` returns early only with a fresh boundary.
- So `stale_boundary` is the collector's verdict: `applyClaudeCompact` saw `compact.Timestamp.Before(preAt)`, with preAt being the PreCompact row's created_at (14:44:43Z).
- One likely cause is that the boundary timestamp reaches the collector as the zero time after the hook → collector encoding (`harnez_context`, `codexEvidence.Compact`, broker/socket transport). An off-by-one created_at, or the wrong PreCompact row, is also possible. Unverified.

The real payload keys are confirmed: `compact_summary, cwd, hook_event_name, prompt_id, session_id, transcript_path, trigger`.

## M1d pre-work (fix the collector comparison)

1. A test that round-trips a real-shaped payload through the same encoding the hook uses to reach the collector: `PrepareCodexTelemetry` → the bytes sent over the broker/socket → `ProcessCodexTelemetryTx`.
   - Use the keys above and a boundary 35 s after the PreCompact row.
   - It must fail on c3bdc52b with `stale_boundary`.
2. Add `boundary_at` and `precompact_at` to the diagnostic note: timestamps only, no content.
3. Fix it. The live note must then say `applied`, and the PostCompact row must carry `input_tokens`/`total_tokens` = postTokens and the model.
4. Gates: one `HTO=0 make test-q1`, logging to `q1-1303-m1d.log`. Commit as `fix(compact): … (issue 1303 M1d)`. Never amend. `make install`, then stop; the host runs the live check.

## M1d review (host): Claude writes the boundary after PostCompact returns

M1d delivered (only this compaction's boundary is accepted): ce7279ff. q1: 83 ok, 0 FAIL.

Live run, session `eeaf12bb-…`:
- **Rows:** PreCompact row 822 at 15:03:19.417Z; PostCompact row 823 at 15:03:44.815Z. Still empty.
- **Hook:** PostCompact took **3.683 s** (the full `boundary_wait`). The note says `stale_boundary` on the hook side; `precompact_at` and `boundary_at` are null.
- **Transcript:** the boundary line carries `timestamp` 15:03:44.488Z (pre 160680). The file's **mtime is 15:03:49.081Z**, after the hook finished at about 15:03:48.48Z.

Conclusion (verified by timing on one run, consistent with the three earlier runs):
- Claude Code stamps the boundary before PostCompact but appends it to the transcript only after the PostCompact hook returns.
- Polling inside the hook can never see it; it only delays Claude by `boundary_wait`.
- The earlier 68 ms run saw the previous compaction's boundary.

## Decision (decide mode, M1d review)

Fill the Claude PostCompact row in the collector, after the hook returns: a deferred fill.
- The collector is long-running. When a Claude postcompact arrives without a fresh boundary, it schedules re-reads of `transcript_path`, with the delays in spec.
- When the boundary that follows the PreCompact row appears, it updates that PostCompact row and its token snapshots and reruns reconcile.
- Remove the in-hook wait, so PostCompact returns fast again.
- Reason: this is the only placement that can see the boundary, and it is reversible.
- A collector restart loses pending re-reads, so the next hook event of that session also triggers a pending fill: the ingest-time retry.

## M1e pre-work (deferred fill)

1. Remove `boundary_wait`/`boundary_poll` from the hook path and the spec. Keep a single read in the hook. Replace with collector retry delays in `spec/hooks.yaml` `claude_compact`, for example `fill_retry: [2s, 10s, 60s]`, with a schema entry. No Go defaults.
2. Collector: on a Claude postcompact row without an applied boundary, retry on that schedule.
   - Apply only a boundary whose timestamp is ≥ that compaction's PreCompact row's `created_at`.
   - Update the existing row, without inserting a duplicate. Write the snapshots and rerun reconcile once.
   - Make it idempotent: a later retry or hook event must not apply it twice.
   - Also attempt pending fills for the session on its next telemetry event.
3. Tests, using the fake clock or scheduler seam with no real sleeps:
   - the boundary appears after the first retry, which fills the row;
   - it never appears, so the row stays empty and exactly N retries run;
   - only an older boundary exists, which is not applied;
   - a duplicate retry does not double-write;
   - a restart, then the next session event, fills it.
4. Diagnostic: the note records `precompact_at` and `boundary_at` for every outcome, plus `filled_after` (the retry index, or `event`).
5. Gates: one `HTO=0 make test-q1`, logging to `q1-1303-m1e.log`. Commit as `fix(compact): … (issue 1303 M1e)`. Never amend. `make install`, then stop for the host's live check.

## M1e review (host): live fill works

M1e delivered (deferred fill in the collector, `fill_retry` in spec): fbe30687. q1: 83 ok, 0 FAIL.

Live run, `harnez agent compact --name dev-1303b`, which reported "150777 -> 12495 tokens":
- **Hook:** PostCompact took 111 ms (no in-hook wait).
- **Notes:** at ingest `stale_boundary` (it saw the previous boundary, 15:10:20Z). Then `applied` at 15:17:13.6Z with `filled_after: retry:1`, `precompact_at` 15:16:28.9Z and `boundary_at` 15:17:11.1Z.
- **Rows:** PostCompact row 836 has `input_tokens`/`total_tokens` 12495 and model `claude-haiku-5-5`. The snapshots are precompact 150777 and postcompact 12495, which match the compact output.
- **Economics row 382:** model `claude-haiku-5-5`, `compaction_cost_micros` 15078, `post_compaction_cost_micros` 1250, status `insufficient_data` ("missing no-compaction baseline", which is expected for Claude).

Implausible value:
- 15078 = 150777 × the base rate 100000 µ$/M.
- `spec/pricing.yaml:14-15` says a call above `tiers[].over_input_tokens` (100k for Haiku 5.5) uses that tier's rates, which is 5x.
- So the compaction cost is under-priced 5x for any Claude compaction above 100k. Every Haiku auto-compaction is one.

Accepted developer choices:
- A note is written for every fill attempt.
- A pending fill is dropped after the 72 s schedule.

Not accepted: the fill's row update, snapshots and reconcile run in separate writes, so a crash can leave a half-filled row.

## M2 pre-work (M1 follow-ups, then the quota sample)

M1 follow-ups:
1. Run the deferred fill's row update, snapshots and reconcile in one transaction. Add a test with a failure injected after the row update: no partial state remains, and the next attempt fills it.
2. Make the economics pricing apply the input tiers from `spec/pricing.yaml`, the same rule the #1302 report uses. Test: a 150777-token Haiku precompact is priced at the tier rate, and a 90k one at the base rate. Use the near-miss values 100000 and 100001.

M2 proper (quota sample per compaction), per "Decision (decide mode, plan review)":

3. On a Claude compaction, the collector stores one quota sample in `turn_quota_boundaries` (`internal/usagestore/store.go:680`), boundary `compact`, with `captured_at`, `cache_age_ms`, `has_cache` and `error`.
   - Use the cache first. Do a live fetch only when the cache is older than the spec age.
   - The fetch runs under a cross-process, non-blocking file lock. If the lock is held, record the cached sample with its age and do not fetch.
   - It runs in the collector, never in the hook, so it never delays Claude.
4. Move `TurnQuotaTimeout` (`internal/usage/turnquota.go:31`) and the new cache age into `spec/usage.yaml` with schema entries. No Go defaults.
5. Tests use the `turnQuotaHTTPClientFactory` seam and a fake clock, with no sleeps:
   - a fresh cache, so no fetch;
   - a stale cache with the lock free, so one fetch;
   - a stale cache with the lock held, so the cached sample is stored and there is no fetch;
   - a fetch timeout, so the error is recorded and the compaction rows are unaffected;
   - two compactions, so two samples.

   Tests never reach the live collector or the real usage endpoint.
6. Name the #1302 consumer in `docs/Telemetry.md`: how the report pairs the API cost between two compaction samples with the quota rise. The report change itself is out of scope unless trivial.
7. Gates: one `HTO=0 make test-q1`, logging to `q1-1303-m2.log`. One or two commits, `fix(compact): …`/`feat(compact): … (issue 1303 M2)`. Never amend. `make install`, then stop for the host's live check.

## M2 review (host): tiers, atomic fill, quota sample work live

M2 delivered: 6345ca28 (input tiers in economics, atomic fill) and f833d576 (one quota sample per Claude compaction). The spec keys are `turn_quota.timeout` 1800ms and `turn_quota.compact_cache_max_age` 5m. q1: 83 ok, 0 FAIL. The changed assertions are equivalent and use the spec value; none is weakened.

Live run, "118926 -> 11802 tokens":
- **Rows:** PostCompact row 851 has 11802 tokens and model `claude-haiku-5-5`. The fill note says `applied`, `retry:1`.
- **Economics:** `compaction_cost_micros` 59463 = 118926 × 500000 µ$/M, which is the tier rate (5x base) above 100k. It was 15078 before the fix.
- **Quota sample:** `turn_quota_boundaries` 2533, boundary `compact`, source `compact-fetch`, fresh, `import_key compact-record:…`. The linked observation 202709 has 5h at 0.10 and weekly at 0.79, which matches `harnez usage`.

Accepted caveats:
- The sample goroutine is lost if the collector exits mid-sample.
- The 401 fallback to `claude -p /usage` is the shared fetch's existing behaviour.
- AGY's timeout is still a constant, which is out of scope.

## M1 one-shot review (neon:med, 3m10s): findings

A neon:med reviewer covered `bc074577..fbe30687`. Host check:
- **HIGH privacy, confirmed in code** (`codex_telemetry.go:52`): PostCompact payloads keep `compact_summary`/`custom_instructions`. They are sent to the collector and, if it is unreachable, spooled to disk by `PostTelemetry` (`client.go:43`).
  - The live spool files checked are command-run records, not compaction payloads; their key names were checked, not their values.
- **HIGH:** fill timers are not stopped on collector shutdown.
- **MED:**
  - A sidechain or other-session boundary can be applied (`claude_compact.go:88`).
  - The transcript read is unbounded.
  - Fire and on-event fills are not serialized; a corrupt pending file is never removed; orphaned pending files are never removed.
  - `TestClaudeFillWritesOnceAcrossRepeatedRetries` does not reach the `total_tokens IS NULL` guard.
- **LOW:** hook tests lack `isolateHookEnv`; ns vs ms timestamp comparison.

## M3 pre-work (review fixes)

1. Drop `compact_summary` and `custom_instructions` from the payload before `json.Marshal` in `PrepareCodexTelemetry`, so they are never posted or spooled. Test: the prepared bytes contain neither key, but evidence still records `Claude=true`.
2. Fill lifecycle:
   - Keep the timers in the server and stop them in shutdown, with a test.
   - A second PostCompact for a session replaces (stops) the earlier timers.
   - One mutex serializes fire and on-event.
   - Remove a corrupt pending file, writing a diagnostic note.
   - On collector start, drop pending files older than the schedule.
3. `scanClaudeCompact`: skip sidechain boundaries and boundaries whose `sessionId` is not the hook's session. Tests: near-miss boundaries for sidechain, foreign session and older session.
4. Bound the transcript read with a spec key, for example a `claude_compact.max_transcript_bytes` that reads only the tail, with a schema entry. Compare timestamps at ms precision, truncating the PreCompact time to ms. Test the equal-ms case.
5. The repeat-retry test must reach the `total_tokens IS NULL` guard: restore the pending file, then refire, and assert one write. Add `isolateHookEnv` to `cmd/harnez/claude_compact_hook_test.go`.
6. Gates: one `HTO=0 make test-q1`, logging to `q1-1303-m3.log`. Commit as `fix(compact): … (issue 1303 M3)`. Never amend. `make install`, then stop.
