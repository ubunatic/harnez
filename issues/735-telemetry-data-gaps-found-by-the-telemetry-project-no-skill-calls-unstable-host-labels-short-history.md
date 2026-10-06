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
