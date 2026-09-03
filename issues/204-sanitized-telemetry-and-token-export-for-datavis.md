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
- **Level 2 — Agent-Sanitized Prose (LLM-in-the-loop / Incremental Cleaner)**:
  - When human-readable context or notes *are* desired on demand (e.g. high-level summaries of what an agent did), use an agent/LLM-based cleaning pipeline:
    - **Prompt Contract**: Rewrites or summarizes delicate tool notes into sanitized, high-level abstract descriptions (e.g., "Refactored UI component styling" instead of "Fixed internal auth bug in client Acme Corp repo").
    - **Incremental Processing & Content-Hash Caching**: Never re-process unchanged text. Compute `sha256(raw_note)` (or row ID) and store sanitized counterparts in a local translation/cache table (`note_sanitization_cache: raw_hash -> clean_text`).
    - **Incremental Stream**: When new telemetry rows arrive, only new un-sanitized texts are dispatched to the agent cleaning step before export.
- **Level 3 — Internal / Scrubbed**:
  - Retain structural fields, but regex-scrub paths (`/home/<user>/...` -> `~/...`), emails, and API keys.
- **Level 4 — Raw**:
  - Full unscrubbed export for local/private backup.
- **Output Formats**:
  - **JSON**: Compact aggregate timeseries & breakdown records suitable for static dashboard loading.
  - **SQLite**: A clean, single-table/relational file with sensitive fields redacted or omitted, usable client-side via WebAssembly SQLite (`sql.js`).

## 3. Implementation Plan

1. Define export schema and privacy scrubbing rules in `internal/telemetry/export.go` and `internal/usage/export.go`.
2. Implement CLI subcommand (e.g. `harnez telemetry export` or `harnez usage export`).
3. Add unit and integration tests verifying that paths like `/home/uwe/...` and email addresses are scrubbed from exported payloads.

## 4. Progress / Scope Note

First pass (2026-09-03) implemented **JSON export only**:

- `harnez usage export --out=<file> [--db <path>] [--history-dir <dir>]`, nested under
  `usage` (`cmd/harnez/usageexport.go`), writing a single envelope combining both
  telemetry and usage-history exports.
- `internal/telemetry/export.go`: `BuildExport`/`ExportAll` transform `tool_calls` rows
  into `ExportToolCall` records.
- `internal/usage/export.go`: `BuildUsageExport`/`ExportHistory` transform merged
  `HistoryEntry` records into `ExportPoint` records.

**SQLite export is deferred** to a follow-up ticket — not implemented, not stubbed.

Fields scrubbed and how:

- `WorkingDir` / `ProjectName` / `TicketID` (telemetry): reduced to `filepath.Base(...)`
  only (e.g. `/home/uwe/projects/harnez` -> `harnez`) via `normalizeProjectPath`, applied
  uniformly to all three fields in case any of them ever holds a path rather than a
  short identifier. Exported as `project_dir` / `project_name` / `ticket_id`.
- `Account` (usage, e.g. an email): re-run through the existing `MaskAccount` helper
  (`internal/usage/util.go`). Every current producer already masks `Account` before it
  is set, so this is a defensive re-application (idempotent on an already-masked value),
  not a new masking scheme.
- `Hostname` (usage-history): **not** passed through only `sanitizeHostname` — that
  helper (`internal/usage/history.go`) just makes a value filesystem-safe (lowercase,
  disallowed characters swapped for `-`); it does not anonymize, and a real hostname
  routinely embeds a username (e.g. `uwes-workstation.local`). Instead each real
  hostname is mapped to an opaque, per-export-run label (`host-1`, `host-2`, ...)
  assigned in first-seen order (`anonymizeHostname` in `internal/usage/export.go`),
  using `sanitizeHostname`'s output only as the map key so formatting differences don't
  split one machine into two labels. This keeps per-machine trends distinguishable in
  the exported timeseries without leaking the real name.
- `Note` (telemetry, free text) and `Sources`/`Details` (usage, free-form
  strings/maps): dropped entirely rather than exported. Both are unstructured
  human/tool-written text that has historically held paths or other identifying
  content, with no safe automatic way to scrub arbitrary free text.
- `SessionID`: kept as-is. It is produced by `internal/resolve.Session()`, not derived
  from username, hostname, or email — reasoned to carry no PII on its own. Worth
  revisiting if `resolve.Session`'s derivation ever changes to embed anything
  identifying.

Tests (assert by string search over the actual serialized JSON bytes, not just that a
scrub function was invoked):

- `internal/telemetry/export_test.go`: `TestBuildExport_ScrubsWorkingDir`,
  `TestBuildExport_ScrubsAbsolutePathTicketAndProject`, `TestBuildExport_DropsNote`,
  `TestExportAll`.
- `internal/usage/export_test.go`: `TestBuildUsageExport_ScrubsAccountAndHostname`,
  `TestBuildUsageExport_DropsSourcesAndDetails`,
  `TestBuildUsageExport_StableHostLabels`, `TestExportHistory`.
- `cmd/harnez/usageexport_test.go`: `TestRunUsageExport_EndToEndScrubsRawPII` (full
  CLI-layer round trip against a temp `~/.harnez`-style fixture: sqlite db + jsonl
  history file, checking the written output file),
  `TestRunUsageExport_ToleratesMissingFixtureDirs`.

`go test ./...` passes except one pre-existing, environment-dependent flaky test
(`TestBuildWatchFrameCompactAllUsageDoesNotStarveLoad` in `internal/usage`, unrelated to
this ticket — it reads the real local `~/.claude/harnez/usage-history` directory's live
size into a layout-width assertion) confirmed to fail identically with or without this
ticket's changes present.

Independent security review of the scrubbing logic is still pending, per this ticket's
Status remaining Open (not Closed).

Note on scope vs. §2's privacy-level scheme: this pass does not implement the `--privacy`
flag or Levels 1-4 (including the Level 2 LLM-in-the-loop note-sanitization pipeline with
its content-hash cache) — it implements a single fixed behavior equivalent in spirit to
"drop the free-text `note`/`Sources`/`Details` fields, scrub everything else," closest to
§2's Level 1. The privacy-level flag, per-level behavior, and the LLM-based note cleaner
remain open follow-up work alongside SQLite export.
