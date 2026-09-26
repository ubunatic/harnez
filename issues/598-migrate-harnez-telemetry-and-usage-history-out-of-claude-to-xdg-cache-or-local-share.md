# 598 — Migrate harnez telemetry and usage history out of ~/.claude to XDG ~/.cache or ~/.local/share

**Status**: Open
**Priority**: P1 (High)
**Severity**: Moderate
**Category**: Architecture / Telemetry / Storage
**Related**: [515](515-one-discoverable-home-for-all-harnez-telemetry-data.md), `internal/telemetry/telemetry.go`, `internal/usage/history.go`, `internal/usage/claude.go`, `docs/Telemetry.md`

---

## Goal

`/goal`: Decouple Harnez state, telemetry, and usage history from Claude-specific directories (`~/.claude/harnez/`) and establish a single, standard XDG-compliant storage location (e.g. `~/.local/share/harnez/` for durable databases and `~/.cache/harnez/` for transient caches) across all providers (Claude, AGY, Codex).

## 1. Problem & Context

Harnez was originally incubated as a Claude extension, which left several core telemetry and state files residing inside Claude's configuration tree:
- `~/.claude/harnez/usage-history/*.jsonl` (quota history and per-host usage snapshots)
- `~/.claude/harnez-quota-cache.json` (cached quota state)
- Inconsistent DB naming: `~/.harnez/tool_catalog.sqlite` holds general tool and CLI telemetry rather than tool definitions alone.

As Harnez now orchestrates multiple independent agent harnesses (Claude, Antigravity/AGY, Codex), storing core Harnez data inside `~/.claude/` is architecturally incorrect, confusing to non-Claude agents, and prone to permission/path mismatches.

## 2. Proposed Architecture & Target Layout

### Standard XDG Directory Split
1. **Durable Data / Databases (`$XDG_DATA_HOME/harnez` or `~/.local/share/harnez/`)**:
   - `telemetry.sqlite` (or `telemetry.db`): Consolidated SQLite database for `tool_calls`, `cli_invocations`, and session records (migrated from `~/.harnez/tool_catalog.sqlite`).
   - `usage-history/`: Durable quota snapshots and per-host history JSONL files (migrated from `~/.claude/harnez/usage-history/`).
   - `voice-history/` or voice recordings.
2. **Transient Caches (`$XDG_CACHE_HOME/harnez` or `~/.cache/harnez/`)**:
   - `quota-cache.json`: Provider quota cache (migrated from `~/.claude/harnez-quota-cache.json`).
3. **Configuration & Rules (`$XDG_CONFIG_HOME/harnez` or `~/.harnez/config.yaml`)**:
   - User-level configurations and rule overlays.

### Automatic Migration & Fallback
- On startup, Harnez checks for existing files at legacy locations (`~/.claude/harnez/usage-history/`, `~/.harnez/tool_catalog.sqlite`).
- Automatically migrates existing records to the new target location without data loss.
- Legacy paths are removed or symlinked during migration to avoid decoy zero-byte files.

## 3. Sprint Milestones

- **M1 — Path Resolution & Auto-Migration**:
  - Update `internal/telemetry.DefaultDBPath()` to resolve to `$XDG_DATA_HOME/harnez/telemetry.sqlite` (fallback `~/.local/share/harnez/telemetry.sqlite`).
  - Update `internal/usage` history directory default from `~/.claude/harnez/usage-history` to `$XDG_DATA_HOME/harnez/usage-history`.
  - Update quota cache path from `~/.claude/harnez-quota-cache.json` to `$XDG_CACHE_HOME/harnez/quota-cache.json`.
  - Implement zero-loss auto-migration from legacy paths on first access.
- **M2 — Callsite Updates, Verification & Docs**:
  - Update all CLI defaults (`--history-dir`), docs (`docs/Telemetry.md`), and tests.
  - Add regression/migration tests verifying automatic data transfer and XDG environment overrides.
  - Verify with `make test-q1`.
- **M3 — Independent Review Gate**:
  - Dispatched reviewer (`terra:med`) audits path resolution, migration safety, and test assertions.
- **M4 — Teardown & Ticket Close**:
  - Final verification, subagent cleanup, and ticket closure.

## 4. Acceptance Criteria

- No telemetry, usage history, or quota cache files are written to `~/.claude/harnez/`.
- `internal/telemetry.DefaultDBPath()` resolves to standard `$XDG_DATA_HOME/harnez/telemetry.sqlite` (or `~/.local/share/harnez/telemetry.sqlite`).
- `internal/usage/history.go` and `internal/usage/claude.go` read and write to the new location.
- Automatic migration test ensures existing history and SQLite records transfer seamlessly.
- Documentation in `docs/Telemetry.md` and CLI flags (`--history-dir`) reflect the updated default paths.

