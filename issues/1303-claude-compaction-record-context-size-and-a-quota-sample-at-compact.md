# 1303 — Claude compaction: record context size and a quota sample at compact

**Status**: Open
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
