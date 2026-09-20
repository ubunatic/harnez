# 435 — Native Subagent Replacement, Harnez Agent Dispatch Interception, and A/B Telemetry Switch

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Agentic Ergonomics / Infrastructure
**Related**: #342, #417, `docs/HarnezAgentArchitecture.md`, `docs/practices/AgenticLoop.md`

---

## 1. Problem & Motivation

Different AI coding harnesses (`agy`, `codex`, `claude`) provide isolated, proprietary native subagent dispatch tools (`invoke_subagent`, `spawn_agent`). These native tools:
1. Cannot cross harness boundaries (e.g. `agy` cannot easily invoke `codex:luna:low`).
2. Hide token velocity and KV cache telemetry from the caller.
3. Lack automatic context compaction policies and reconnectability controls.

To establish `harnez agent` as the standardized cross-harness subagent protocol, we need:
- An **Opt-In/Out Switch** (`subagent_mode: harnez|native`) in `config.yaml` and CLI flags.
- **Native Subagent Interception/Replacement**: When enabled, native `invoke_subagent` / `spawn_agent` calls are intercepted by harness hooks or replaced, returning an explicit instruction directing models to use `harnez agent` along with reference documentation.
- **Comparative A/B Telemetry**: Enable developers to measure and compare token consumption, speed, cache hit rates, and cost between `native` and `harnez agent` modes.

---

## 2. /goal & Acceptance Criteria

### /goal
Implement the `subagent_mode: harnez|native` configuration switch, native subagent tool interception hooks in AGY/Codex/Claude harnesses with redirection banners, and side-by-side A/B telemetry tracking in `tool_catalog.sqlite`.

### Acceptance Criteria
1. `config.yaml` supports `subagent_mode: harnez|native` with CLI overrides `harnez apply --enable-harnez-agent` and `--disable-harnez-agent`.
2. When `subagent_mode: harnez` is enabled, invoking native subagent tools in AGY, Codex, or Claude triggers an interception hook that rejects the call and outputs a structured redirection banner explaining how to use `harnez agent start/resume` and referencing `docs/HarnezAgentArchitecture.md`.
3. Dispatches under both native and `harnez agent` record structured telemetry in SQLite for A/B comparison.
4. An executing agent must verify live codebase status before beginning implementation.

---

## 3. Implementation & Verification Plan

### 3.1 Configuration & Hook Interception
- Add `SubagentMode` to `internal/claude` config structures.
- Implement pre-tool hook in AGY and Codex hook configurations that intercepts `invoke_subagent` / `spawn_agent` when `subagent_mode == "harnez"`.
- Format redirection banner output:
  ```
  [HARNEZ SUBAGENT INTERCEPTION]:
  Native subagent tool is replaced by 'harnez agent'.
  Please use:
    harnez agent start <provider>:<model> "<task_prompt>"
  To resume:
    harnez agent resume <session_id> "<next_prompt>"
  See docs/HarnezAgentArchitecture.md for full details.
  ```

### 3.2 A/B Telemetry Recording
- Capture `dispatch_mode` (`harnez` vs `native`) in `cli_invocations` and `tool_calls` tables.
- Add comparative summary metrics in `harnez stats` / `harnez assess`.

### 3.3 Automated Verification Target
- Unit tests in `internal/claude/` and `cmd/harnez/` verifying flag parsing, hook generation, and interception behavior.
- Live probe test verifying hook rejection of native subagent calls when enabled.
