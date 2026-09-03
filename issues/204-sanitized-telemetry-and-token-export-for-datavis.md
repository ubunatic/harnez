# 204 — Sanitized Telemetry & Token Export Subcommand for Visual Analytics

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [ubunatic.com/issues/029-agent-usage-and-telemetry-visualization.md](../../ubunatic.com/issues/029-agent-usage-and-telemetry-visualization.md), [120-harnez-stats-analytical-reporting.md](120-harnez-stats-analytical-reporting.md), [048-usage-history-subcommands-refactor.md](048-usage-history-subcommands-refactor.md), [116-tool-telemetry-schema-and-storage-layer.md](116-tool-telemetry-schema-and-storage-layer.md)

---

## 1. Problem & Motivation

`harnez` records granular tool execution telemetry in `~/.harnez/tool_catalog.sqlite` and token/session timeline snapshots across machines in `~/.claude/harnez/usage-history/*.jsonl`.

External visualizations (such as personal dashboards or public/portfolio websites like `ubunatic.com`) want to present these metrics:
- Token consumption trends over time across all agents (Claude, Codex, AGY).
- Tool invocation distribution, success vs. failure rates, and distillation byte savings.
- Adherence to the visual exploration mantra: *"Overview first, details on demand"*.

However, exporting the raw SQLite or JSONL directly poses serious privacy and security risks:
1. **Delicate Prose & Business Secrets in Tool Notes**:
   - The `note` column in `tool_calls` (and potential tool arguments/prompts) contains free-form text written by agents (e.g. `note: "git status: clean tree...", "filed issue 128...", customer repo details, internal architectures"`). In commercial, proprietary, or client projects, these notes can reveal intellectual property, commercial roadmaps, commit hashes, client names, or secret internals.
2. **System & Identity Leakage**:
   - `tool_calls` and usage snapshots store real absolute filesystem paths (`working_dir`), system usernames, account emails, hostnames, and session IDs.
3. **Missing Unified Export Format & Anonymization Policies**:
   - There is no mechanism to selectively strip free-form text or export purely numerical/categorical aggregations.

## 2. Proposed Solution & Architecture

Add a sanitized export mechanism to `harnez`:
- E.g. `harnez export telemetry --out=<file> [--format=json|sqlite] [--privacy=public|internal|raw]`

### Privacy / Anonymization Levels:
- **Level 1 — Public / Zero-Prose (Default for web datavis)**:
  - **Completely drop free-form prose**: omit `note`, arguments, and command lines entirely.
  - **Bucket or generalize categorical fields**: map `project_name` to an opt-in allowlist or generic aliases (`project-a`, `project-b`), strip `working_dir`, strip `session_id` and replace with salted ephemeral session hashes if session grouping is needed.
  - **Aggregate metrics only**: timestamp (rounded to hour/day), `agent_id`, `tool_name`, `call_type`, `score`, exit code / success bool, durations, token counts, and distillation byte savings.
- **Level 2 — Internal / Scrubbed**:
  - Retain structural fields, but regex-scrub paths (`/home/<user>/...` -> `~/...`), emails, and API keys.
- **Level 3 — Raw**:
  - Full unscrubbed export for local/private backup.
- **Output Formats**:
  - **JSON**: Compact aggregate timeseries & breakdown records suitable for static dashboard loading.
  - **SQLite**: A clean, single-table/relational file with sensitive fields redacted or omitted, usable client-side via WebAssembly SQLite (`sql.js`).

## 3. Implementation Plan

1. Define export schema and privacy scrubbing rules in `internal/telemetry/export.go` and `internal/usage/export.go`.
2. Implement CLI subcommand (e.g. `harnez telemetry export` or `harnez usage export`).
3. Add unit and integration tests verifying that paths like `/home/uwe/...` and email addresses are scrubbed from exported payloads.
