# 052 — Research: Headless CLI status probes to refresh telemetry when agents are idle

**Status**: Closed — research complete
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

## Findings (2026-08-29)

Investigated on the local machine with all three CLIs installed
(`claude` at `/home/uwe/.local/bin/claude`, `codex` at `/home/uwe/.local/bin/codex`,
`agy` — actually invoked via the shell alias `agy='ANTIGRAVITY_AGENT=1 agy'` pointing
at `/home/uwe/.local/bin/agy`, a stripped Go binary; matches the process-name
substrings `internal/usage/agy.go`'s `findAGYPorts` greps for: `"agy"` and
`"antigravity"`).

### Premise check: do claude/codex even need a headless probe?

Read `internal/usage/claude.go` and `internal/usage/codex.go` in full before running
anything. Both already bypass the "ask the CLI to refresh its local cache" problem
entirely — `CollectClaude` and `CollectCodex` make their own direct, live HTTPS
calls (`https://api.anthropic.com/api/oauth/usage` with the bearer token from
`~/.claude/.credentials.json`; `https://chatgpt.com/backend-api/wham/usage` with
the bearer token from `~/.codex/auth.json`) whenever `harnez` runs, using the
long-lived OAuth/refresh token cached on disk. Neither path is gated on
`CountRunningAgentProcesses()` or on an interactive session being open — a fully
idle machine with no `claude`/`codex` process running still gets a live quota
reading on every `harnez usage` invocation, subject only to the existing
`harnez-quota-cache.json` sibling-process debounce (issue 033). So Research
Question 1/3 ("headless probe to refresh telemetry when idle") **does not apply**
to Claude or Codex — there is no CLI-refresh step to trigger because harnez never
depended on the CLI's own cache for quota in the first place; it talks to the
provider APIs directly.

Confirmed neither `claude doctor` nor `codex doctor` (the only two "headless status"
style subcommands either CLI exposes — see below) touch quota data at all; both
report install/environment health only (versions, paths, DB integrity, etc.), not
rate-limit state.

### Per-CLI command survey (ran `--help` for real, then ran the candidates)

- **claude** (`claude --help`, 274 lines): no `status`/`usage`/`quota` subcommand.
  Only `doctor` matches the "health check" shape. Ran `claude doctor` — non-
  interactive, exits immediately (~1s), prints CLI version/update channel/install
  path, explicitly states "No installation issues found" and points to `/doctor`
  inside a session "for a full setup checkup that can also fix issues." No quota
  or telemetry-cache side effect observed; it is a static self-diagnostic, not a
  network probe.

- **codex** (`codex --help`): has `doctor` too. Ran `codex doctor` — non-
  interactive (~1.8s), emits a large environment/install/state-DB health report
  (SQLite integrity, rollout counts, `~/.codex/log` presence, etc.). Also no
  quota/rate-limit data. `codex doctor --json` exists (redacted machine-readable
  report) but same scope. No `codex status`/`codex quota`/`codex usage` subcommand
  exists in this build (0.150.1); the only place live rate-limit data appears is
  the wham API `CollectCodex` already calls directly.

- **agy** (`ANTIGRAVITY_AGENT=1 agy --help`): subcommands are `agent`/`agents`
  (list agents), `changelog`, `help`, `install`, `mcp`, `mic-serve`, `models`,
  `plugin`/`plugins`, `update`. No `status`, `doctor`, `auth`, `quota`, or `usage`
  subcommand exists. Ran `agy models` (lists available models via a network call,
  zero cost, non-interactive, exits in ~1s, exit code 0) as the closest headless
  probe candidate — confirmed it does **not** leave a listener behind (`ps aux`
  immediately after shows no agy process), so it cannot be chained into
  `findAGYPorts`/`QueryAGYLocalQuota`, which require a *running* process holding
  an open local RPC port. `strings` on the `agy` binary confirms the quota RPC is
  `.../v1internal:retrieveUserQuotaSummary` (`RetrieveUserQuotaSummary`) — the
  exact same internal Connect-RPC method `QueryAGYLocalQuota` already calls
  against the live process's local port; there is no separate headless/CLI-level
  entry point for it. Conclusion: **AGY has no non-interactive quota probe today**.
  Getting a quota reading requires an actual running `agy` process (interactive
  session or background agent) that has bound its local LanguageServer RPC port —
  exactly what `internal/usage/agy.go` already relies on, and exactly the gap this
  ticket set out to close. It is not closeable by shelling out to the CLI; it
  would require either (a) a supported public quota REST endpoint from
  Antigravity/Gemini (none found), or (b) deliberately spawning a short-lived
  headless `agy` process for the sole purpose of quota polling, which is a much
  larger and riskier change (process lifecycle, zero-token guarantee on an actual
  agent binary, resource usage per poll) than "probe before reading cache."

### Zero-token/zero-cost guarantee (Research Question 2)

`claude doctor`, `codex doctor`, and `agy models` are all read-only/self-
diagnostic or list calls — none dispatch a synthetic prompt or touch a model
endpoint that would be billable. This guarantee holds for the three commands
actually available, but none of them produce quota data, so the guarantee is
moot for the ticket's actual goal.

### Conditional execution / remote applicability (Research Questions 3–4)

Moot for claude/codex per the premise check above (no gating on process count
needed — they always fetch live). For AGY, `CountRunningAgentProcesses()`
gating is already how `internal/usage/agy.go`'s live-quota path effectively works
(no running process → `findAGYPorts()` returns nothing → `QueryAGYLocalQuota` is
never called → quota fields are simply absent from that collection). There is no
headless command to insert into that gap. On a remote host (e.g. `um760`) the
same conclusion applies over SSH: `claude`/`codex` telemetry is already live and
host-independent (subject to network reachability to the provider APIs), while
AGY quota on a remote host still requires an actual running `agy` process there.

### Architecture proposal for `ProbeIdleAgents(ctx)`

Not building it. Given the findings above there is no `internal/usage/*.go`
change this ticket can responsibly propose:
- Claude and Codex need no probe — they already refresh live and independently
  of process state.
- AGY has no headless CLI probe to shell out to; closing its idle-refresh gap
  would require spawning a real `agy` process or a new provider-side REST
  endpoint, both out of scope for a "shell out to a status subcommand" design and
  each deserving its own dedicated, separately-scoped ticket if pursued.

### Conclusion

Research questions answered; no viable headless-probe mechanism exists to
implement across any of the three agents (moot for claude/codex, unavailable for
agy). Closing as research-complete. If idle-AGY-quota freshness becomes a
priority, the next actionable step is a *new* ticket scoped to "spawn a
short-lived headless `agy` process to poll quota" (with its own zero-token-cost
verification against a live process, not just `--help`), not a `ProbeIdleAgents`
wrapper around existing CLI subcommands.
