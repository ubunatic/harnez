# 052 — Research: Headless CLI status probes to refresh telemetry when agents are idle

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Research
**Related**: [internal/usage/process.go](file:///home/uwe/projects/harnez/internal/usage/process.go), [internal/usage/remote.go](file:///home/uwe/projects/harnez/internal/usage/remote.go), [issues/049-running-agent-processes-watch-panel.md](file:///home/uwe/projects/harnez/issues/049-running-agent-processes-watch-panel.md), [issues/050-remote-host-flag-and-watch-hotkey.md](file:///home/uwe/projects/harnez/issues/050-remote-host-flag-and-watch-hotkey.md)

## Summary

Investigate non-interactive / headless command-line invocations for `claude`, `agy`, and `codex` to refresh local cache files and quota state when no active interactive agent session is running.

Currently, telemetry collectors rely on local cache files (e.g. `~/.claude/`, `~/.gemini/antigravity-cli/`, `~/.codex/`) that are primarily kept fresh when an agent session is actively open. Users often keep idle terminal sessions running solely to keep telemetry accurate on local and remote machines.

## Research Questions & Invariants

1. **CLI Probe Availability**:
   - What non-interactive commands or flags exist for each agent (e.g., `claude status`, `claude --version`, `agy auth check`, `codex quota`) that trigger internal telemetry/quota cache updates?
   - Do these commands execute without launching an interactive REPL or blocking on standard input?

2. **Zero-Token & Zero-Cost Guarantee**:
   - Ensure the candidate probe commands NEVER dispatch synthetic user prompts, consume token quotas, or increment billable usage.

3. **Conditional Execution Flow**:
   - When `harnez usage` (or remote `CollectRemote`) detects 0 running processes for an agent (via `CountRunningAgentProcesses()`), conditionally execute the headless probe before reading the local cache.
   - Implement rate-limiting / debounce (e.g., at most once every 5–15 minutes per agent) to prevent excessive subprocess spawning.

4. **Remote Host Applicability**:
   - Verify how headless probing improves accuracy on remote hosts (e.g., `um760`) without requiring persistent background agent sessions.

## Deliverables

1. Canary audit (`scripts/canary-agent-probes.sh` or research notes) evaluating CLI flags and behavior across all 3 agents.
2. Architecture proposal for `ProbeIdleAgents(ctx)` in `internal/usage/`.
