# Webpage Screenshot Inspection Telemetry Study

**Date:** 2026-10-06
**Related:** [Issue 737](../../issues/737-measure-webpage-screenshot-inspection-costs-and-reduce-guidance-if-warranted.md), [doc-image benchmark (387)](../../issues/387-benchmark-token-efficiency-of-doc-screenshots-and-images-vs-raw-markdown-text-across-agent-harnesses.md)
**Status:** Preliminary local telemetry research

<!-- harnez:topic: Local telemetry evidence and attribution gaps for webpage screenshot inspection -->

## Question

Can local telemetry show how often agents capture and view webpage screenshots, their elapsed time, and the token cost attributable to those inspections? The earlier document-card study is background only: it does not measure webpage screenshot workflows.

## Data coverage and reproducible queries

Read-only database: `~/.local/share/harnez/telemetry.sqlite`. The `tool_calls` table covered 2026-08-31 09:34:15 UTC through 2026-10-06 20:05:18 UTC (140,727 rows when queried). It records tool name, provider/agent, session, timestamp, duration, optional input/output/total/actual token fields, and a free-form note. It has no model, asset path/type, browser/page identifier, or screenshot-capture field. This is one machine's retained data, not a representative sample of all agents or webpage work.

Reproduce the exact image-view count/provider coverage:

```sql
SELECT agent_id, tool_name, call_type, COUNT(*) AS calls,
       COUNT(DISTINCT session_id) AS sessions,
       MIN(created_at), MAX(created_at), SUM(duration_ms),
       SUM(actual_tokens), SUM(total_tokens)
FROM tool_calls
WHERE tool_name = 'view_image'
GROUP BY agent_id, tool_name, call_type;
```

Reproduce the bounded daily Codex view counts:

```sql
SELECT substr(created_at, 1, 10) AS day, COUNT(*) AS calls,
       COUNT(DISTINCT session_id) AS sessions, SUM(duration_ms) AS duration_ms
FROM tool_calls
WHERE agent_id = 'codex' AND tool_name = 'view_image'
GROUP BY day ORDER BY day;
```

Candidate capture-command search (recall is limited by note contents):

```sql
SELECT agent_id, tool_name, call_type, COUNT(*) AS calls,
       COUNT(DISTINCT session_id) AS sessions, SUM(duration_ms), SUM(actual_tokens)
FROM tool_calls
WHERE lower(note) LIKE '%screenshot%'
   OR lower(note) LIKE '%view_image%'
   OR lower(note) LIKE '%capture screenshot%'
GROUP BY agent_id, tool_name, call_type ORDER BY calls DESC;
```

The targeted raw-log check opened one Codex rollout corresponding to a high-activity session and searched for image-view records. Its retained conversation covered other repository work; screenshot-related words also appeared in pasted instructions and retrospective notes. This demonstrates that keyword hits are not a reliable screenshot-capture classifier. No broader transcript scan was run.

## Findings

- **Recorded image views:** `view_image` has 705 Codex rows across 120 sessions, between 2026-09-18 and 2026-10-06. One Claude `view_image` row appears on 2026-10-04. No AGY tool is named exactly `view_image`. These are tool invocations, not confirmed webpage screenshots; they can include diagrams, app screenshots, photos, or other images.
- **Observed frequency:** Codex daily counts vary from 1 to 290 in the queried window; 2026-10-06 has 290 rows across 14 sessions, while 2026-10-05 has 61 across 9. This shows image viewing can be frequent in some sessions, but does not establish repeated webpage inspection. Duplicate telemetry hooks or replayed records may also affect counts; deduplication by execution identifier was not established.
- **Screenshot captures:** Not reliably countable. Capture often happens within shell/browser commands recorded generically (for example, `Bash`); the database does not preserve a consistent command/asset classification for this query. The note keyword query returns incidental prose and image-related code, so its 28 AGY hits are candidates only, not verified captures.
- **Elapsed time:** `duration_ms` totals 0 for the `view_image` rows. That means this telemetry does not measure image-view latency; it does not mean viewing took no time. A few capture-command candidates have command durations, but they do not consistently identify webpage screenshots, and the time may include rendering, waiting, and unrelated command work.
- **Token usage:** The 705 Codex rows have `actual_tokens` populated (sum 11,996,134) and `total_tokens` populated (sum 74,419,288). These values repeat session/context snapshots on view events; summing them across events double-counts context. Sample rows within one session repeat the same 17,257 actual / 33,354 total values. They are observed session-context snapshots, not token increments or image-attributable usage. No defensible image token estimate can be derived from them. The Claude row has no token counts.
- **Provider/model comparison:** Provider/tool labels support the counts above, but not a fair provider comparison: only Codex has meaningful image-view volume in this table, with a single Claude event and no equivalent AGY event. The schema lacks model, so these counts cannot be split by model. Per-session Codex rollout metadata may sometimes reveal a model, but it would not solve image-token attribution.

## Interpretation and recommendation

The data supports that image viewing is identifiable for Codex and can be frequent in individual sessions. It does **not** establish the webpage screenshot subset, screenshot-capture frequency, elapsed viewing time, or attributable tokens. Therefore these measurements do not justify reducing screenshot guidance on cost grounds, and they also do not establish that routine screenshot inspection is expensive. Keep the issue open for follow-up evidence; do not change guidance based on this preliminary query.

For a conclusive follow-up, instrument a stable screenshot/view event with session, provider/model, asset kind, capture-versus-view phase, start/end time, and the provider's per-turn token delta or an explicit `unattributed` value. Compare a bounded set of webpage sessions and distinguish captures, actual image deliveries to the model, and repeated inspection turns.

## Limitations

- Local, time-bounded telemetry only; no sampling of sessions without Harnez hooks or other machines.
- Tool names identify a viewer in some clients but do not establish image subject or whether an image reached model context.
- Capture calls are mixed with generic shell/browser commands; transcript keyword search is noisy and was intentionally limited.
- Token columns are session/context observations, not per-image billing. Summed values are reported only to show the tempting but invalid aggregate.
- No elapsed duration is captured for image-view events, and there is insufficient comparable Claude/AGY data or model attribution.
