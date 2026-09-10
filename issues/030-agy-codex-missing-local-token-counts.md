# 030 — AGY and Codex have no local token-count source

**Status**: Closed — Codex rollout token aggregation is complete in 36de25e; AGY token extraction is deferred to #034 because its local data remains opaque protobuf
**Category**: Feature gap
**Discovered**: 2026-08-18, while adding a `[T]` token-count toggle to `harnez usage --watch` panels

---

## Problem

### Audit — 2026-09-10

- **Conclusion: unsolved implementation; source discovery complete for Codex.**
  `CollectCodex` reads configuration, authentication, and quota data, but never
  sets `AgentUsage.Tokens`. `CollectAGY` also leaves it unset. The only usage
  collector assigning a `TokenBreakdown` remains Claude; no rollout/token_count
  parser is wired into `internal/usage`.
- **Reported evidence:** the advisor reuse study
  [records local Codex rollout counters](../docs/studies/2026-09-10-advisor-session-reuse-and-cross-agent-dispatch.md).
  Thus the original absence-of-local-source statement is historical, not a
  current blocker. Inspecting advisor stats manually did not implement #030.
- **Measured:** `go test ./...` passes. Codex collector tests cover quota
  assignment, cache age, and concurrent fetch suppression; they contain no
  multi-rollout numeric aggregation or malformed-record regression tests.
  The Codex implementation plan remains pending; AGY extraction remains
  unimplemented and related to #034. No transcript content was read in this audit.

### Implementation progress — 2026-09-10

- Codex rollout JSONL totals are now aggregated defensively, deduplicated by
  session, and exposed as `AgentUsage.Tokens` (`36de25e`). Malformed records
  remain nonfatal.
- AGY protobuf extraction remains deferred to #034, so this ticket stays Open
  with the Codex half complete.

`harnez usage` (and its `--watch` live view) shows a `TokenBreakdown` (input/output/cache/total tokens) for Claude Code, but never for Antigravity (AGY) or OpenAI Codex. This isn't a missing UI toggle — there is currently no local data source to populate it from:

- **Claude Code**: totals come from a pre-aggregated local file, `~/.claude/stats-cache.json` (`internal/usage/claude.go`). Easy.
- **Antigravity (AGY)**: the live LanguageServer RPC (`RetrieveUserQuotaSummary`, see `internal/usage/agy.go`) only returns quota *percentages*, never raw token counts. The actual usage data lives in per-conversation SQLite databases (`~/.gemini/antigravity-cli/conversations/*.db`, tables `steps`/`gen_metadata`) as opaque **protobuf blobs** — no `.proto` schema is available locally, so parsing them means reverse-engineering the wire format field-by-field.
- **OpenAI Codex**: the live `wham/usage` endpoint (`internal/usage/codex.go`) is also percentage-only. On this machine, `~/.codex/` doesn't even contain session rollout/transcript files to sum locally (it's mostly plugin/skill caches, not conversation history) — there may be nothing to parse even if we tried.

## What was checked

- `~/.gemini/antigravity-cli/conversations/*.db` — confirmed schema (`trajectory_meta`, `steps`, `gen_metadata`, `executor_metadata`, ...); `steps.step_payload` and `gen_metadata.data` are `BLOB` columns, almost certainly protobuf-encoded, not JSON.
- `~/.codex/` — no `sessions/`, `rollout*`, or similar transcript directories present; `logs_2.sqlite` contains structured tracing/debug logs (INFO/TRACE spans) but no token-usage fields found via `LIKE '%token%'` search.

## Possible paths forward (not started)

1. Reverse-engineer the AGY protobuf schema well enough to extract per-turn token counts from `gen_metadata`/`step_payload` — likely the higher-value target since the data clearly exists locally, just encoded.
2. Check whether a newer/different Codex CLI version writes session rollouts locally (this machine's `~/.codex/` may just not have them enabled) before assuming the data doesn't exist at all.
3. As a fallback, accept percentage-only display for AGY/Codex (already works) and drop the expectation of raw token counts for these two agents rather than investing in protobuf reverse-engineering.

## Related

- [Issue 023: `harnez usage`](023-usage-command-token-quota-tracking.md) — original command this extends
- [docs/studies/2026-08-18-usage-watch-tui-terminal-rendering-postmortem.md](../docs/studies/2026-08-18-usage-watch-tui-terminal-rendering-postmortem.md) — session this was discovered in

---

## Implementation Plan

**The Codex half of this ticket is now unblocked.** Re-probed while planning
(2026-09-04): `~/.codex/sessions/YYYY/MM/DD/rollout-<ts>-<uuid>.jsonl` now
exists on this machine (it did not when the ticket was filed) and carries
**plain-JSON** cumulative token counts — no protobuf, no reverse engineering:

```jsonc
{"timestamp":"2026-09-03T20:08:47.765Z","ordinal":1270,"type":"event_msg",
 "payload":{"type":"token_count","info":{
   "total_token_usage":{"input_tokens":14698822,"cached_input_tokens":14500608,
     "cache_write_input_tokens":0,"output_tokens":53722,
     "reasoning_output_tokens":14274,"total_tokens":14752544},
   "last_token_usage":{...}}}}
```

The first line of each rollout is a `session_meta` record with `session_id`,
`cwd`, `cli_version`, and `model_provider` — enough for per-project attribution.
AGY remains protobuf-only (confirmed: `gen_metadata.data` blobs start `X'1204…'`,
classic protobuf wire format, no `.proto` locally).

So: **do Codex now, defer AGY, drop nothing.** Ticket's option 2 is confirmed
true; option 1 stays deferred; option 3 applies to AGY only.

### Steps (Codex)

1. **`internal/usage/codex.go`** — add `collectCodexTokens(codexDir string) (*TokenBreakdown, []string)`:
   - Walk `~/.codex/sessions/**/rollout-*.jsonl`. Cap the walk by mtime window
     (e.g. rollouts touched in the last N days, N from the caller) so the cost
     doesn't grow without bound as history accumulates.
   - For each file, **read backwards** for the last `event_msg`/`token_count`
     record rather than parsing the whole file — `total_token_usage` is
     cumulative per session, so only the final one matters. A tail-read of the
     last ~64KB covers it in practice; fall back to a full scan only if no
     `token_count` is found in the tail.
   - Sum `total_token_usage` across sessions into `TokenBreakdown`:
     `InputTokens` ← `input_tokens`, `OutputTokens` ← `output_tokens`,
     `CacheReadTokens` ← `cached_input_tokens`,
     `CacheWriteTokens` ← `cache_write_input_tokens`, `TotalTokens` ← `total_tokens`.
     `reasoning_output_tokens` has no `TokenBreakdown` field — either fold it
     into `OutputTokens` or surface it via `Details["reasoning_tokens"]`;
     prefer `Details` so the existing four-way breakdown keeps its meaning.
     Leave `CostUSD` at 0 (no local price table for Codex).
   - Append each parsed rollout path family to `usage.Sources` as a single
     summarizing entry (`~/.codex/sessions (N rollouts)`), not N paths.
2. **Wire into `CollectCodex`** (`internal/usage/codex.go:117`): set
   `usage.Tokens` after the existing quota fetch, mirroring how
   `internal/usage/claude.go:110-137` sets it from `stats-cache.json`. Keep it
   non-fatal — a parse failure must degrade to today's percentage-only display,
   never error the whole collector.
3. **Cache it.** Scanning rollouts on every `--watch` tick is wasteful. Reuse
   the existing snapshot machinery in `internal/usage/statecache.go`
   (`WriteAgentSnapshot` / `ReadAgentSnapshot` / `cacheOrLive`) rather than
   introducing a new cache file; the token sum rides along in the persisted
   `AgentUsage`. Only recompute when the newest rollout's mtime is newer than
   the snapshot's `FetchedAt`.
4. **Tests** — `internal/usage/codex_test.go`: build a temp `sessions/` tree
   with two synthetic rollout files (one with several `token_count` records, one
   with none), assert the summed `TokenBreakdown` fields exactly (not just
   non-nil), assert the no-`token_count` file contributes zero rather than
   erroring, and assert a malformed JSON line is skipped without failing the
   collector.
5. **UI**: no change expected — the `[T]` toggle in `harnez usage --watch`
   already renders `usage.Tokens` when non-nil. Verify with `make smoke` /
   a live `harnez usage --json | jq '.[] | select(.agent_id=="codex") | .tokens'`.

### AGY

Leave unimplemented in this ticket; note that
[034](034-hook-triggered-token-extraction.md) proposes the better route
(hook payload → `transcript.jsonl` seek) which avoids the protobuf entirely.
Update this ticket's status to reflect "Codex done, AGY tracked in 034" rather
than closing it outright.

### Design decisions / tradeoffs

- **Backwards tail-read over full parse**: rollouts reach multi-MB; the counter
  is cumulative, so the last record is the only one needed. Worth the modest
  complexity.
- **Cumulative-per-session summing double-counts nothing** as long as each
  rollout file is one session — confirmed by `session_meta.session_id` being
  unique per file. Guard anyway: de-duplicate by `session_id`, since Codex has
  historically written resumed sessions into new files with the same id.
- **No new cache format** (contra issue 034's `~/.cache/harnez/usage.json`);
  `statecache.go` already exists and is what `usage`/`--watch` read.

### Risks / open questions

- Codex rollout schema is undocumented and version-dependent
  (`cli_version: 0.153.0` here). Parse defensively: unknown/missing fields →
  zero, never an error. Consider recording the observed `cli_version` in
  `Details` so a future schema break is diagnosable from `harnez usage --json`.
- Privacy: rollouts contain full conversation text. The parser must extract
  only the numeric `token_count` payload and must never copy message content
  into snapshots or exports — cross-check `internal/usage/privacy_export_test.go`
  expectations before landing.
- Unbounded history growth makes a naive walk slow; the mtime window plus the
  snapshot cache are load-bearing, not optional.

### Scope

**Medium** for Codex (one new parser + cache wiring + tests, ~200 lines).
**Large / deferred** for AGY (protobuf reverse engineering — do not start here).
