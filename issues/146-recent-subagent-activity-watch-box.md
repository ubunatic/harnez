# 146 — Assess recent subagent activity in compact usage watch

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [[049-running-agent-processes-watch-panel]], [[082-agent-usage-collector-daemon]], [[108-subagent-dispatch-sequential-default-and-issue-number-race-guard]], [[145-orchestrator-session-skill-and-command]], `internal/usage/watch.go`

---

## 1. Problem & Motivation

`harnez usage --watch --compact` shows broad agent activity but does not tell
the user whether currently active coding tools have spawned subagents, or
which subagents were active recently. That obscures delegated work that is
often the most relevant live state during an orchestrated coding session.

Add a compact TUI box, when feasible, showing subagent activity for active
coding tools within a recent window such as the last hour.

## 2. Technical Specification / Findings

- Assess available integration points for agent-management state from Codex,
  Claude Code, AGY, and other supported coding tools. Prefer supported APIs,
  local state, or process metadata over transcript scraping.
- Define a normalized subagent activity record: parent tool/session, agent
  identity or task label, model when available, lifecycle state, and last
  activity timestamp.
- The display should use a bounded rolling window (default candidate: one
  hour), be resilient to absent integrations, and avoid falsely presenting
  ordinary agent processes as verified subagents.
- Decide whether a compact-watch box can fit without breaking existing layout
  rules, and specify omission/summary behavior in narrow terminals.

## 3. Implementation & Verification Plan

- Produce a feasibility matrix for each supported coding tool, documenting
  data source, freshness, permissions, privacy considerations, and fallback.
- Design the subagent-activity data model and collection cadence; reuse the
  existing usage collector/watch refresh architecture where appropriate.
- Implement an optional compact TUI box with recent active/completed
  subagents, clearly distinguishing unknown or unavailable sources.
- Add fixture-driven tests for window filtering, lifecycle aggregation,
  no-data behavior, and compact-layout overflow; manually verify against a
  live multi-agent session.

---

## Implementation Plan

### Feasibility findings (done as part of this planning pass)

- **Process metadata is a dead end for subagents.** `internal/usage/process.go`
  counts `claude`/`agy`/`codex` processes from `/proc`. Claude Code subagents run
  *in-process* inside the host `claude` — they never appear as a distinct
  process. So `AgentProcessCount` cannot distinguish delegated work, and issue
  [[049]]'s existing Processes box already shows everything /proc can offer.
- **Telemetry cannot see it either, yet.** `tool_calls` (`internal/telemetry/schema.go`)
  has a `tool_name` column, so a `Task` row would be exactly the signal wanted —
  but rows are only written by agent-initiated `harnez rate` calls; the
  PostToolUse auto-capture that would populate them mechanically is [[124]],
  still Open. Nothing in `internal/` currently reads agent session transcripts
  (`internal/usage/history.go`'s `*.jsonl` handling is harnez's own
  usage-history store, unrelated).
- **The one real local source is the Claude Code session transcript**
  (`~/.claude/projects/<slug>/<session>.jsonl`), whose entries carry an
  `isSidechain` field and whose `Task` tool_use blocks carry `subagent_type`.
  That is transcript scraping, which §2 of this ticket asks to avoid.

**Therefore the honest recommendation is to reframe this ticket**: either
(a) block it on [[124]], which makes `Task` calls a first-class telemetry row and
turns this feature into a straightforward query over an existing table, or
(b) accept a Claude-Code-only, transcript-derived MVP now, clearly labelled as
best-effort. Option (a) is the better engineering: the display is ~100 lines
once the data exists, and 124 is the piece that actually needs building.

### Steps (assuming option (a) — build after 124)

1. `internal/telemetry/query.go` — add
   `QueryRecentSubagents(since time.Time) ([]SubagentActivity, error)`: select
   `tool_calls` rows with `tool_name = 'Task'` within the window, grouped by
   `session_id` + agent, returning parent agent, session, task label (from
   `note`), and last-activity timestamp. `idx_tool_calls_created_at` and
   `idx_tool_calls_tool_name` already exist, so no schema change.
2. `internal/usage/types.go` — add the normalized record (parent tool/session,
   agent identity/task label, model when available, lifecycle state, last
   activity). Model and lifecycle are `omitempty`/unknown-tolerant: do not
   invent values the source cannot supply.
3. `internal/usage/watch.go` — add `buildSubagentsBox(width int, acts []SubagentActivity) wbox`
   alongside `buildProcessesBox` (~line 639), a `watchBoxSymbol` entry, and a
   toggle key in `dispatchWatchKey`/`applyWatchSectionKey`. Follow the existing
   box conventions exactly (`maxPanelContentWidth`, `truncateVisible`,
   `renderWBox`).
4. Section presets: include the box in `defaultWatchSections()` and
   `compactWatchSections()` only if it fits; the existing hidden-count hint
   mechanism (see `watch_test.go` around the "N hidden" assertions) handles
   narrow terminals — reuse it rather than inventing omission logic.
5. Empty/absent-source behaviour: when the window has no rows, the box must
   render a single "no recent subagent activity" line, and when the data source
   is unavailable it must say *unknown*, never zero. Assert both.
6. Tests in `internal/usage/watch_test.go`: fixture-driven window filtering
   (inside/outside the 1h window), lifecycle aggregation (multiple calls in one
   subagent collapse to one row), no-data line, unknown-source line, and
   compact-layout overflow. Plus a `internal/telemetry` query test.
7. Manual verification against a live multi-agent session (the ticket asks for
   this explicitly and the fixtures cannot substitute for it).

### Design decisions

- Reuse the telemetry DB and the existing watch refresh cadence; do not add a
  collector or a new store for this.
- Rolling window default 1h, as a constant, not a flag — add a flag only if the
  default is observed to be wrong.
- Never present an ordinary agent process as a verified subagent: the Processes
  box and the Subagents box stay separate, and the Subagents box renders only
  rows with a positive dispatch signal.

### Risks / open questions

- **Codex and agy have no known local subagent state at all.** The box will show
  Claude Code only for the foreseeable future; it must label that limitation
  rather than implying the other agents spawned nothing.
- Privacy: task labels come from agent-written notes and may contain project
  text. Route them through the existing note-sanitization path
  (`internal/telemetry/sanitize_cache.go`) if the box is ever exported.
- Vertical space in `--compact` is already contested; this box may be
  hidden-by-default with a toggle key rather than shown by default.

### Scope

Medium if [[124]] lands first (query + one box + tests). Large if built
standalone, because it then requires a transcript-parsing subsystem this repo
does not have and explicitly prefers not to add.
