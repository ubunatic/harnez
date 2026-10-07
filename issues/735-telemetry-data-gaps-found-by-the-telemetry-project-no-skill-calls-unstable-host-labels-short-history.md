# 735 — Telemetry data gaps found by the telemetry project: no Skill calls, unstable host labels, short history

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Telemetry
**Related**: issues/720 (split), `~/projects/telemetry` issues 001 (import) and 002 (reports), `docs/Telemetry.md` §4.1, `cmd/harnez/usageexport.go`

---

## 1. Problem & Motivation

The new telemetry project (`~/projects/telemetry`) imports `harnez usage export`. Its first look at
a real export (2026-10-06, 40 MB, 81,983 tool calls, 6,336 usage points) found three gaps that
block or weaken its reports. The owner wants to see which skills they call often (telemetry 002).

## 2. Technical Specification / Findings

1. **No Skill calls.** No tool name in the export contains "skill". The Claude hooks harnez
   installs match only `Bash|View|Read|ReadMultipleFiles|Edit|Write`, so `Skill` calls never
   reach the store. Codex and other agents were not checked. Needed: record skill invocations
   with the skill name in a field (and for slash commands the user types, if the agent exposes
   them).
2. **Host labels shift between exports.** The sanitizer replaces host names with `host-1`,
   `host-2`, ... in order of first appearance in that export. When old history drops out, labels
   can change, and the telemetry project's usage-point key `(timestamp, hostname, agent_id)`
   would mix up machines. Needed: a label that stays the same across exports (e.g. a salted hash),
   without revealing the host name. This changes a field's meaning: bump `format_version`.
3. **Short history.** Tool calls cover only 2026-09-26 to 2026-10-06, and usage points stop on
   2026-09-16. No export window or prune was found for `tool_calls` in a quick look
   (`cli_invocations` has a row cap). Find out why: store reset, migration, collector stopped,
   or export filter. If usage history stopped being recorded on 2026-09-16, that is a bug.

## 3. Implementation & Verification Plan

/goal A fresh `harnez usage export` contains skill calls with their names, host labels that are
the same across exports, and the full stored history (or a documented reason for the gap); or stop
and report when blocked on an owner decision or a denied permission.

Check the live code and recent commits first. Items can be done separately; item 3 starts with
an investigation.

## Finding 4 (telemetry 001 import, 2026-10-06)

Every usage point appears exactly three times in the export: 6,336 points, 2,112 distinct by
`(timestamp, hostname, agent_id)`. The telemetry import collapses the copies, but harnez writes or
exports each point three times. Find where (history writer, collector, or export).

## Progress

### M1 — usage points stored three times (fixed)

Cause: the store, not the export. Older importers keyed each legacy history line by its source
(`~/.local/share/harnez/usage-history/<host>.jsonl:<line>`, `~/.claude/harnez/usage-history/...`,
and a line hash), so the same snapshot became three rows in `usage_summary_observations`.
Fix: importers use the canonical key `hostname:observed_at`; a one-time cleanup
(`usage-summary-dedupe-v1`, `usagestore.DedupeUsageSummaries`) keeps the oldest row per
(hostname, observed_at). Live DB 2026-10-06: 2,154 rows before, 1,436 deleted, 718 after; a real
export then had 2,112 points, 2,112 distinct by (timestamp, hostname, agent_id). An export with no
usage points now writes `"points": []` instead of `null`.

### M2 — short history (fixed where recoverable)

Tool calls: not a retention limit. The XDG move (`telemetry.DefaultDBPath`) renamed
`~/.harnez/tool_catalog.sqlite` to `.bak-<time>` without copying its rows whenever the new
`telemetry.sqlite` already existed. The move now merges legacy `tool_calls` first, and
`harnez stats merge-legacy` recovers older backups in one transaction after a `VACUUM INTO` backup.
Live run 2026-10-06: 83,988 rows before; backup
`~/.local/share/harnez/telemetry.sqlite.before-merge-20261006T183921Z`; 55,615 added from
`bak-20260927T095508`, 0 from the six other timestamped backups; 139,603 after. No duplicate
groups were created (the three same-second pairs left already exist inside the legacy file).
A real export now has tool calls from 2026-08-31T09:34Z. `bak-issue331-20260914131025` is excluded
from the default set: its 93 extra rows are the junk signatures issue 331 deleted on purpose.
Not merged from the backups: `cli_invocations`, `compaction_events`, `session_boundaries`,
`token_snapshots`, `compaction_economics` and caches.

The larger history broke the export (`too many SQL variables` in the note-category cache lookup);
the lookup now runs in batches of 500.

Usage points: commit c2a6d7c7 dropped history recording from `usage --watch`, so nothing but
`harnez usage history record` wrote snapshots (t14 stopped 2026-09-03, x600 2026-09-16, UM760
2026-08-23). `harnez usage` and `usage --watch` now record one snapshot per host every
`history_snapshot_interval` (15m, `spec/usage.yaml`); verified live (t14 snapshot at
2026-10-06T18:40Z, a second run within 15m added none). Gap: snapshots between those dates and
2026-10-06 were never stored and cannot be recovered.

### M3 — skill calls and typed /commands recorded (fixed)

Claude `Skill` tool calls recorded with the skill name (new `skill_name` column, export `skill` field);
failed calls recorded as failed. Plugin names (`plugin:skill`) kept intact.
Typed `/name` prompts recorded when matching installed skills/commands; paths (`/tmp/a.md`, `/home/x`)
and built-ins (`/clear`, `/compact`, `/model`) excluded. Probe (`scripts/canary-735-skill-hooks.sh`) confirmed
that Claude built-ins do not reach `UserPromptSubmit`.
The prompt hook runs on every prompt via stat checks only without DB scans (benchmark: ~9.3 µs/op).
Codex and AGY heuristic skill calls recorded with `tool_name = "Skill"`, `call_type = "skill-inferred"`,
and `skill_name = <name>`.
Old rows keep no skill name (empty string `""`).

### M4 — stable HMAC host labels and format_version 2 (fixed)

`usage.points[].hostname` uses stable pseudonyms (`host-` + first 8 hex chars of HMAC-SHA256(canonical hostname, key)).
32-byte secret key stored with `0600` permissions at `$XDG_DATA_HOME/harnez/export-host-key` (configurable via `export_host_key` in `spec/usage.yaml`).
Export `format_version` bumped to 2; `docs/Telemetry.md` §4.1 updated with key location and format version 2 semantics.
Golden test fixtures updated for format version 2.

## Handoff (host, 2026-10-07)

M1 (4d850285, usage points stored once), M2 (f2ea0c1e, history restored, pre-XDG tool calls
recovered), M3 (skill calls), and M4 (format_version 2, stable HMAC host labels) are complete.
Ready for host review of M1–M4.


