# 756 — harnez channel list should list only active agents by default, with more brevity

**Status**: Closed — Defaulted harnez channel list to active sessions only, added --all flag, and trimmed table columns
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: issues/754-add-filtering-and-projection-flags-to-harnez-channel-list-live-names-ids-with-overview-first-design.md

---

/goal Make the default `harnez channel list` show only active agents in a brief overview, or stop and report when blocked on a user decision or denied permission.

## 1. Problem & Motivation
Issue 754 made live filtering opt-in (`--live`), so the default `harnez channel list` printed stale Claude endpoints and historical Codex threads, plus a `WORKING_DIR` column not needed to pick a target.

## 2. Resolution (commit ceb34426)
- `cmd/harnez/channel.go:117`: the live filter now applies unless `--all` is set. `--live` was removed; no callers remained in code, docs or scripts.
- `filterLiveChannelSessions` (`cmd/harnez/channel.go:141`) keeps only Claude sessions with `PID > 0` and a URL. Dead Claude PIDs are already pruned by the endpoint registry (`internal/channel/registry.go`).
- Codex liveness question, settled by the code: **every** Codex thread is excluded by default, whether or not its process runs; they appear only with `--all` (status `inactive`). Codex discovery has no liveness check.
- Overview columns are now `PROVIDER NAME ID STATUS`; the detail hint footer is kept.
- `--json`, `--names` and `--ids` apply the same default filter.

## 3. Verification
- `go test ./cmd/harnez -run 'TestChannelList'`: `TestChannelListCommandFlags` asserts that `list`, `--names`, `--ids` and `--json` omit the Codex row `codex-old`, and that each `--all` variant includes inactive Codex rows; `TestChannelListOverviewAndLiveFiltering` asserts the 4-column header.
- No test in `internal/channel` changed; none was needed, as the filter lives in `cmd/harnez`.
- Follow-up if live Codex threads must show by default: add a Codex liveness check (new ticket).
