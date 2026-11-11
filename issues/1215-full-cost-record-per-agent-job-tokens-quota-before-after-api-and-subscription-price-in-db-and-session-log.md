# 1215 — Full cost record per agent job: tokens, quota before/after, API and subscription price in DB and session log

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 445, 446, 985, 534, 564, 1213, 1214

---

## 1. Problem & Motivation

Owner idea (2026-11-10): every agent job should carry its full cost, so the owner can see what a job
cost in API money and in subscription quota without reconstructing it from several stores. Today the
numbers are split over `TurnRecord` (session JSON), the turn quota readings and the
`compaction_economics` table, and none of them holds a price per job.

Wanted per job: tokens in, out, cache read, cache write and reasoning; model and effort; quota before and
after; API list price (from `spec/pricing.yaml`, ticket 445); subscription price share (plan price times
quota percent used). Written to the database and to the session log.

## 2. Findings (Jules leads, checked at 45147c93)

From `reports/jules/job-cost-record.md` (GitHub #169, 5 of 5 references held):

1. `TurnRecord` (`internal/subagent/session.go:89-96`) has new input, cached input and output tokens, but
   no reasoning tokens, although `TurnResult` has `ReasoningTokens` (`internal/subagent/driver.go:289`).
   It also has no cache-write tokens (host finding).
2. `TurnRecord` stores no cost and no pricing revision, so a cost computed later uses today's prices.
3. `Session` has no total cost for cheap reporting.
4. `TurnQuotaReading` (`internal/usage/turnquota.go:15-24`) has no subscription price share, and plan
   prices are not in `spec/` yet.
5. `compaction_economics` (`spec/telemetry.yaml:185-195`) stores rates and costs but no effort and no raw
   token counts.

Open facts: how each provider reports cache-write and reasoning tokens (canary per provider, see 1214 for
Codex counting errors), and the owner's plan prices.

## 3. Goal

/goal Every finished agent turn records tokens (in/out/cache read/cache write/reasoning), model, effort,
quota before/after, API cost with pricing revision, and subscription share, in the session JSON and the
telemetry DB, with unknown values stored as unknown, never 0; tests pin each field from a real captured
sample. Stop and report when a provider gives no such number or when plan prices need an owner decision.
On a loop or no progress, clear the goal and write a failure report here.

## Owner-idea runs

| idea | kind | session name | Jules ID | PR | state |
| --- | --- | --- | --- | --- | --- |
| job cost record | ideas | jules-ideas-job-cost-record | 5645731545976621196 | github #169 (closed) | done, 5/5 refs hold; findings above |
| per-call context toggles | ideas | jules-ideas-context-toggles | 8738282058587509182 | none | failed on the Jules side, no PR |
| preamble per model family | ideas | jules-ideas-preamble-per-family | pending | | running |
| tool-only mode envelope | experiment | jules-experiment-tool-only-envelope | 18278899567407530536 | github #175 (closed) | done: works; host reran TestParseEnvelope at the PR head (pass). Lead: the lenient parser (first `{`, last matching `}`) handles prose around the JSON; risk: picks a wrong object when the prose holds braces. Report `reports/jules/tool-only-envelope.md`; code not merged || tool-only mode plug-in point | ideas | not started | | | queued |
