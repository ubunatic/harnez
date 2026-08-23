# 048 — Refactor usage history flags into subcommands

**Status**: Closed — resolved
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Architecture
**Related**: [cmd/harnez/main.go](file:///home/uwe/projects/harnez/cmd/harnez/main.go), [internal/usage/history.go](file:///home/uwe/projects/harnez/internal/usage/history.go), [issues/023-usage-command-token-quota-tracking.md](file:///home/uwe/projects/harnez/issues/023-usage-command-token-quota-tracking.md)

## Summary

The `harnez usage` command currently houses point-in-time snapshots, watch/summary TUI, and history features (`--history`, `--timeline`, `--fetch`) all behind flags on a single command. As history features expand (fetching, timeline rendering, stats, retention), this creates flag conflicts and bloated options.

Because the history feature is brand new and internal, backward compatibility is not required. All history-related flags on `harnez usage` should be removed and structured as subcommands under `harnez usage history`.

## Requirements

1. **Remove deprecated flags from root `usage` command**:
   - Remove `--timeline`, `--history`, and `--fetch` flags from `harnez usage`.
   - Remove the mutual exclusion check code from `usageCmd.RunE`.

2. **Add `harnez usage history` Subcommands**:
   - `harnez usage history` (default / timeline):
     - Displays the merged usage history timeline across all recorded machine logs (previously `harnez usage --timeline`).
     - Supports `--json` flag for JSON timeline output.
   - `harnez usage history fetch <host>`:
     - Fetches remote usage history JSONL files from an SSH host into `~/.claude/harnez/usage-history/` (previously `harnez usage --fetch <host>`).
   - `harnez usage history record`:
     - Explicitly records a snapshot of current usage to local machine history.
   - `harnez usage history stats`:
     - Displays aggregate statistics (file counts, token totals, burn rates, sparkline) across recorded history using `usage.HistorySummaryStats`.

3. **Subcommand Registration**:
   - Register `historyCmd` and its child commands under `usageCmd` in `cmd/harnez/main.go`.
   - Ensure all subcommands have clean `--help` docstrings and handle errors idiomatically.

4. **Tests & Build**:
   - Verify `go test ./...` passes.
   - Verify `make install` succeeds.
   - Test subcommands via CLI execution.
