# 515 — One discoverable home for all harnez telemetry data

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Architecture
**Related**: `docs/Telemetry.md`, `internal/telemetry/telemetry.go`, `internal/usage/history.go`, [[507-capture-subscription-quota-snapshots-at-harnez-agent-start-resume]], [[113-record-collector-roundtrip-times-usage-meta]]

## Problem

On 2026-09-23 an opus investigator asked "why did one astra turn jump the Codex quota?"
and concluded "no data": it queried empty or stale stores while the collector behind
`harnez usage --compact --watch` had recorded every reading. Stores as found:

| Path | State |
|---|---|
| `~/.harnez/tool_catalog.sqlite` | the real telemetry DB (22 MB, live), name doesn't say "telemetry" |
| `~/.harnez/telemetry.sqlite` | 0 bytes since 09-16 (decoy) |
| `~/.local/share/harnez/telemetry.db` | 0 bytes since 09-18 (decoy) |
| `~/.claude/harnez/usage-history/quota-history.jsonl` | live quota readings (~3 min cadence) |
| `~/.claude/harnez/usage-history/<host>.jsonl` | per-host snapshots, last write 09-16 |
| `~/.harnez/sessions`, `~/.harnez/agents` | session state, agent sessions |
| `~/.local/share/harnez/voice-input/history.jsonl` | voice history |

`harnez usage history timeline` reads the stale per-host files, not `quota-history.jsonl`,
and `harnez stats` covers tool calls only. The measured answer (11 luna turns < 1 point of
the 5h window, one astra turn +2 points) was only found by grepping `quota-history.jsonl`.

## /goal

All harnez telemetry data (tool catalog, quota history, session and agent records,
collector snapshots) lives under one root directory with a documented layout, and it is
hard to look in the wrong place:

- One root (e.g. `~/.harnez/data/`) with per-component subdirectories. Components stay
  separate files or DBs ("soft" connection), but share one root, one naming scheme and
  common join keys (timestamp in UTC, host, agent/provider, harnez session id).
- Empty or stale leftover stores are removed or migrated, never left as decoys; names
  say what they hold (no "tool_catalog" for general telemetry).
- One discovery entry point, e.g. `harnez data` (or `harnez stats --where`), that lists
  every store, its path, size, newest record time and what questions it answers.
  Investigators (human or agent) start there.
- Readers use the live stores: `harnez usage history` includes `quota-history.jsonl`.

## Open questions (verify live code first)

- Root location and XDG compliance (`~/.harnez` vs `~/.local/share/harnez`), and the
  migration path for existing data, including remote hosts (`usage-history` sync).
- Whether quota history should move into SQLite or stay JSONL next to it.
- Which component owns which store, per the component system's separation rules.

## Done when

- `docs/Telemetry.md` shows the single layout and the discovery command.
- The discovery command lists all stores with newest-record times; a test covers it.
- No zero-byte or orphaned store remains after migration; a test or check fails if a
  known store path is empty while its component is active.
- The 2026-09-23 question ("quota before/after one agent turn") is answerable by
  following the discovery output alone.
