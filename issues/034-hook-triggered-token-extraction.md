# 034 — Hook-Triggered File Seek for Local Token Extraction (AGY & Claude)

**Status**: Open  
**Category**: Architecture / Telemetry  
**Related**: [Issue 023: `harnez usage`](023-usage-command-token-quota-tracking.md), [Issue 030: Missing Local Token Counts](030-agy-codex-missing-local-token-counts.md), [Issue 035: Transparent HTTP Proxy Sidecar](035-transparent-proxy-quota-and-token-sidecar.md), [Study: Agent Telemetry](../docs/studies/2026-08-19-agent-telemetry-hooks-proxies-and-log-extraction.md)  

---

## 1. Problem & Context

Issue 030 identified that `harnez usage --watch` only displays a `TokenBreakdown` (input, output, cache read/write) for Claude Code, because Claude aggregates lifetime stats into `~/.claude/stats-cache.json`. AGY and Codex lack a ready-to-read pre-aggregated file:
- **AGY**: Raw token usage exists in local conversation databases (`~/.gemini/antigravity-cli/conversations/*.db`) as Protobuf BLOBs and in workspace transcript files (`.gemini/antigravity/transcript.jsonl`), but querying these via background polling or crawling is heavy, noisy, and requires reverse-engineering binary formats.
- **Polling disk logs**: Running continuous file watchers (`inotify` or poll loops) across arbitrary project directories introduces background CPU drain and file descriptor leaks.

## 2. Proposed Solution: Hook-Triggered File Seek

Lifecycle hooks in Antigravity (`hooks.json`) and Claude Code (`~/.claude/settings.json`) provide synchronous lifecycle events whenever a turn completes. Although hooks do not pass token numbers directly in their stdin JSON payload, they provide exact contextual pointers:

1. **Antigravity (`hooks.json`)**:
   - `PostInvocation` and `Stop` hooks pass `conversationId`, `stepIdx`, and `transcriptPath` (e.g. `/path/to/project/.gemini/antigravity/transcript.jsonl`).
   - A lightweight hook command (e.g., `harnez ingest-hook --agent agy`) receives this payload, seeks to the specific line/step in `transcript.jsonl`, parses the step's token metadata (or calculates diffs), and appends the usage to the shared cache (`~/.cache/harnez/usage.json`).

2. **Claude Code (`settings.json`)**:
   - `Stop` or `PostToolUse` hooks fire at turn completion with session ID and project context.
   - Claude's per-project JSONL transcript (`~/.claude/projects/<slug>/<session-id>.jsonl`) can be read at the exact trailing offset without scanning entire directory trees.

## 3. Advantages

- **Zero Polling / Zero Crawling**: Runs purely on event dispatch when the LLM generates tokens.
- **Accurate Attribution**: The hook payload provides `conversationId`, workspace path, and step index, allowing metrics to be attributed to specific projects and subagents.
- **Lightweight & Sub-millisecond**: Seeking to a specific line in a JSONL file takes <1ms and does not add noticeable latency to agent execution.

## 4. Scope & Limitations

- **Tokens Only**: This mechanism extracts token consumption (input, output, cache); it cannot extract API quota windows (5h/weekly limits or remaining requests), which are returned solely in HTTP response headers.
- **Codex Limitation**: OpenAI Codex CLI does not provide a standard declarative lifecycle hook mechanism, so Codex requires either the proxy approach ([Issue 035](035-transparent-proxy-quota-and-token-sidecar.md)) or remote API polling.

## 5. Implementation Plan

1. **Subcommand `harnez ingest-hook`**:
   - Add hidden/internal command `harnez ingest-hook --agent [agy|claude]` that reads hook JSON on stdin, extracts `transcriptPath` + `stepIdx`, updates the local shared usage cache, and writes `{}` to stdout.
2. **Hook Template Injection in `harnez apply` / `harnez init`**:
   - For AGY: Inject a `PostInvocation` hook into `.agents/hooks.json` or global config.
   - For Claude: Inject a `Stop` hook into `~/.claude/settings.json`.
3. **Connect to `harnez usage`**:
   - Read token totals from `~/.cache/harnez/usage.json` in `internal/usage/agy.go` to populate the `[T]` token toggle in `harnez usage --watch`.
