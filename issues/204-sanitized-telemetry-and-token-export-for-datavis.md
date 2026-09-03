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

However, exporting the raw SQLite or JSONL directly poses two issues:
1. **Privacy & Security Leakage**: `tool_calls` and usage snapshots store real system paths (`working_dir`), system usernames, account emails, hostnames, and session IDs.
2. **Missing Unified Export Format**: Tool telemetry and token metrics currently reside in separate stores with separate schemas. There is no command to produce a clean, scrubbed SQLite database or JSON dataset ready for static web consumption.

## 2. Proposed Solution & Architecture

Add a sanitized export mechanism to `harnez`:
- E.g. `harnez export telemetry --out=<file> [--format=json|sqlite] [--anonymize]` (or `harnez usage export`).
- **Anonymization / Scrubbing**:
  - Replace absolute filesystem paths with relative or normalized project identifiers (`ubunatic.com`, `harnez`, `other`).
  - Strip personal email addresses and host credentials.
  - Round timestamps or bin data into hourly/daily aggregations if desired for compact transfer.
- **Output Formats**:
  - **JSON**: Compact aggregate timeseries & breakdown records suitable for static dashboard loading.
  - **SQLite**: A clean, single-table/relational file with sensitive fields redacted or omitted, usable client-side via WebAssembly SQLite (`sql.js`).

## 3. Implementation Plan

1. Define export schema and privacy scrubbing rules in `internal/telemetry/export.go` and `internal/usage/export.go`.
2. Implement CLI subcommand (e.g. `harnez telemetry export` or `harnez usage export`).
3. Add unit and integration tests verifying that paths like `/home/uwe/...` and email addresses are scrubbed from exported payloads.
