# 050 — Support remote host query via `--host` flag and `[r]` hotkey in watch

**Status**: Closed — resolved
**Priority**: P1 (High)
**Severity**: Moderate
**Category**: Feature
**Related**: [cmd/harnez/main.go](file:///home/uwe/projects/harnez/cmd/harnez/main.go), [internal/usage/watch.go](file:///home/uwe/projects/harnez/internal/usage/watch.go), [issues/023-usage-command-token-quota-tracking.md](file:///home/uwe/projects/harnez/issues/023-usage-command-token-quota-tracking.md)

## Summary

Enable querying and monitoring usage from a remote host via SSH by executing `harnez usage --json [--proc]` remotely and rendering the output locally.

Support this both as a direct CLI flag (`harnez usage --host <ssh-target>`) and as an interactive hotkey (`[r]` / `[R]`) in `--watch` mode to toggle/switch between local and remote target views.

## Requirements

1. **Remote Collector (`CollectRemote`)**:
   - Implement `CollectRemote(ctx, host string, includeProcs bool) (UsageSummary, *ProcessCounts, error)` in `internal/usage/`.
   - Executes `ssh -q -o BatchMode=yes -o ConnectTimeout=5 <host> harnez usage --json` (appending `--proc` when requested).
   - Unmarshals remote `UsageSummary` and process metrics cleanly.
   - If SSH fails or `harnez` is missing remotely, return clear error information and set `QuotaFetchError` / placeholder message without crashing.

2. **CLI Flag `--host`**:
   - Add `--host string` flag to `harnez usage` (applies to snapshot, `--summary`, and `--watch`).
   - When `--host` is specified, `CollectRemote` is called instead of local collection.

3. **Interactive Hotkey `[r]` in `--watch` Mode**:
   - Add hotkey `'r'`, `'R'` in `RunWatch`.
   - Allows cycling or switching between local and specified remote host target.
   - Updates the watch header to clearly display the active host target (e.g., `Agentic usage (local)` vs `Agentic usage (@hostname)`).

4. **Testing & Validation**:
   - Unit tests covering JSON decoding of remote responses and error handling on command failures.
   - Verify `go test ./...` and `make install`.
