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
