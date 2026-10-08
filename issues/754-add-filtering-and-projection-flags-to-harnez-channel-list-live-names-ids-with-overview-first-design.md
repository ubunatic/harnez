# 754 — Add filtering and projection flags to harnez channel list (live, names, ids) with overview-first design

**Status**: Closed — Implemented filtering (--live, --names, --ids, --json) and overview-first formatting for harnez channel list
**Priority**: P2 (Medium)
**Severity**: Normal
**Category**: Feature
**Related**: issues/740-claude-plugin-mvp-harnez-agent-send-delivers-via-claude-sendmessage-claude-to-claude.md, issues/752-add-two-agent-lean-sprint-variant-and-rename-legacy-single-dev-loop-to-lean-sprint-v1.md

---

## 1. Problem & Motivation

With the introduction of cross-provider session discovery in `harnez channel list` (merging live Claude endpoints and Codex sessions), the default output dumps all discovered records as raw JSON. In environments with many historical Codex threads, this creates large output payloads that overwhelm terminal users and agent contexts.

Users need:
1. Targeted filtering (e.g. `--live` to restrict to active sessions, or scoping by current directory).
2. Clean projections (e.g. `--names`, `--ids`, or a clean tabular overview rather than monolithic JSON dumps).
3. Adherence to the **"Overview first, details on demand"** UX principle:
   - Default output should be concise, human-readable, and provide a clear hint/footer on how to query full details (e.g. `Use 'harnez channel list --json' or 'harnez channel inspect <id>' for full details`).

## 2. Technical Specification / Findings

- **Code Location**: [`cmd/harnez/channel.go`](file:///home/uwe/projects/harnez/cmd/harnez/channel.go) and [`internal/channel/`](file:///home/uwe/projects/harnez/internal/channel/).
- **Current Behavior**:
  - `harnez channel list` calls `channel.List(ctx)` and `channel.DiscoverCodexSessions(ctx)`.
  - JSON-encodes all returned endpoints unconditionally.
- **Requirements**:
  - Flags:
    - `--live`: Filter to only active/live sessions (running PIDs or active threads).
    - `--names`: Output list of session names only (one per line, token-efficient).
    - `--ids`: Output list of session IDs only.
    - `--json`: Output raw full JSON (preserving existing behavior for programmatic callers).
  - Human-readable default:
    - Compact tabular/bullet overview showing `PROVIDER`, `NAME`, `ID`, `STATUS`, `WORKING_DIR`.
    - Short hint footer indicating how to request full details (`--json` or per-session inspection).

## 3. Implementation & Verification Plan

### Goal
/goal Add filtering and projection flags (--live, --names, --ids, --json) to 'harnez channel list' with an overview-first UX and detail-hint footer, or stop and report when blocked on user input or denied permission.

### Milestones
- **M1: Flag Definition & Filtering Logic**:
  - Add flags to `cmd/harnez/channel.go`: `--live`, `--names`, `--ids`, `--json`.
  - Implement filtering logic in `internal/channel` or command handler.
- **M2: Overview Formatting & Details Hint**:
  - Implement concise default terminal representation with overview-first design and a footer hint for full details.
- **M3: Tests & Verification**:
  - Add unit tests in `cmd/harnez/channel_test.go` covering flag combinations, text output, and JSON mode.
  - Run `make test-q1` and verify with live session calls.
