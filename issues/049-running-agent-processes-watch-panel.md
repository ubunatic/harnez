# 049 — Add running agent processes status box in usage watch/summary

**Status**: Closed — resolved
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: [internal/usage/watch.go](file:///home/uwe/projects/harnez/internal/usage/watch.go), [internal/usage/process.go](file:///home/uwe/projects/harnez/internal/usage/process.go), [cmd/harnez/main.go](file:///home/uwe/projects/harnez/cmd/harnez/main.go)

## Summary

Add a status panel in `harnez usage` (watch / summary) that displays the count and details of running agent processes (`claude`, `agy`, `codex`).
By default, this panel should be hidden, and accessible via:
1. CLI flag on `harnez usage` (e.g. `--proc` / `--processes`) or `--watch --proc`
2. Interactive hotkey in `--watch` mode (`[p]` / `[P]` / key `6`)

## Requirements

1. **Process Discovery**:
   - Implement `CountRunningAgentProcesses()` or `GetAgentProcesses()` in `internal/usage/` (inspecting `/proc` on Linux or fallback to ps) that detects active processes for `claude`, `agy`, and `codex`.
2. **Watch Box Rendering**:
   - Build a `wbox` for processes: title `\x1b[1m[P]\x1b[0m Processes`, listing total active agent processes and per-agent breakdown (e.g., `claude: 1`, `agy: 2`, `codex: 1`).
3. **Toggle & Section Control**:
   - Add `Processes bool` (default `false`) to `watchSections`.
   - Add hotkey `'p'`, `'P'`, `'6'` to toggle the processes panel in `RunWatch`.
   - Update `[a]` (all) / default settings appropriately.
   - Support flag `--proc` / `--processes` on `harnez usage` so `RenderSummary` or `RunWatch` can start with it enabled.
4. **Testing & Validation**:
   - Add unit tests in `watch_test.go` verifying panel rendering, toggles, and process counting logic.
   - Verify `go test ./...` and `make install`.
