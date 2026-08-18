# 030 — AGY and Codex have no local token-count source

**Status**: Open
**Category**: Feature gap
**Discovered**: 2026-08-18, while adding a `[T]` token-count toggle to `harnez usage --watch` panels

---

## Problem

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
