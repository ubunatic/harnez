# 242 — Instruct Agents on `harnez find` for Issue Discovery in `AGENTS.md`

**Status**: Closed
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agent Instructions / Ergonomics
**Related**: [[158-find-entity-query-command]], [[015-agents-md-uman-workspace-awareness]], `AGENTS.md`, `config.yaml`, `docs/templates/AGENTS.md`

---

## 1. Problem & Motivation

Issue 158 introduced `harnez find issues [query]` to provide deterministic, structured issue queries across repository trackers.

However, when 158 was implemented and closed, `AGENTS.md` was not updated to instruct agents about this tool. Consequently, AI agents (Claude Code, Codex, Antigravity) continue falling back to manual filesystem listing (`ls issues/`, grep, or glob searches) when searching for existing issues or allocating next issue numbers.

## 2. Technical Specification

1. **Update `AGENTS.md` (Workspace root & templates)**:
   Add a dedicated section guiding agents on tracker discovery:
   ```markdown
   ## Issue Tracker Discovery (harnez find)

   When searching for existing issues or allocating next ticket numbers, always use
   `harnez find` instead of `ls issues/`, `find`, or raw grep:
   - `harnez find -d <repo> issues status:open` — list active open issues
   - `harnez find -d <repo> issues "<query>"` — fuzzy search across titles and body text
   - `harnez index -d <repo>` — update issues/README.md after filing or updating tickets
   ```

2. **Harness Template Sync**:
   - Update `config.yaml` (`agents_md` templates) and `docs/templates/AGENTS.md`.
   - Run `harnez apply` to synchronize changes across all global and local harness configurations.

## 3. Verification Plan

1. Verify `harnez diff` and `harnez apply` cleanly propagate the instruction.
2. Confirm agents discover and use `harnez find` rather than `ls issues/` in subsequent ticket queries.

## 4. Verification Results

- Updated `/home/uwe/projects/AGENTS.md`, `/home/uwe/projects/harnez/AGENTS.md`, `/home/uwe/projects/harnez/docs/templates/AGENTS.md`, and `/home/uwe/projects/harnez/config.yaml`.
- Added unit tests in `internal/claude/toolfeedback_test.go` verifying `Issue Tracker Discovery` section presence.
- Executed `make test` and `make install` in `/home/uwe/projects/harnez` (all tests passed).
- Ran `harnez diff` (previewed clean additions) and `harnez apply` (updated global instructions `~/.claude/CLAUDE.md` and `~/.prime/agent/AGENTS.md`).
- Confirmed `harnez diff` subsequently reports no changes (idempotent).
- Regenerated issue indices with `harnez index`.
