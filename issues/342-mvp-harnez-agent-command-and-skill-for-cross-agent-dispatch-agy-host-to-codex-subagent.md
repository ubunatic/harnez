# 342 — MVP: /harnez-agent skill and CLI dispatch for agy host to codex:sol subagent

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Ergonomics / Feature
**Category**: Feature / Cross-Agent Dispatch
**Related**: [issues/288](288-add-fresh-codex-skill-lean-fresh-handoff-sprint-using-codex-cli-instead-of-a-claude-subagent.md), [issues/291](291-add-harnez-agent-command-standardized-non-interactive-dispatch-to-external-coding-agent-clis-codex-agy.md)

---

## 1. Goal & MVP Scope

Enable direct cross-agent task delegation from chat using the concise syntax:

```
/harnez-agent codex:sol task:<prose task description>
```

### MVP Scope Focus
- **Primary Host**: Antigravity (`agy`) acting as the interactive Host Orchestrator.
- **Primary Target Subagent**: `codex` CLI (running with `gpt-5.6-sol` model).
- **Other combinations** (`claude` host → `codex`, `agy` → `claude`, `lmcoder` local LLM sandbox) follow in subsequent phases once this core wire is tested and robust.

---

## 2. Syntax & Model Shorthand Resolution

### 2.1 Invocation Syntax
- `/harnez-agent <tool>:<model_shorthand> task:<prose description>`
- E.g.: `/harnez-agent codex:sol task:fix typo in README.md and run tests`

### 2.2 Model Alias Map
The dispatch layer must normalize shorthand vendor names into fully-qualified model identifiers before invoking the target CLI:
- `codex:sol` → `codex` CLI with `--model gpt-5.6-sol`
- `codex:luna` → `codex` CLI with `--model gpt-5.6-luna`
- `codex:terra` → `codex` CLI with `--model gpt-5.6-terra`
- `codex:5.5` → `codex` CLI with `--model gpt-5.5`
- (Future) `claude:sonnet` → `claude` CLI with `--model claude-3-7-sonnet-20250219`
- (Future) `agy:flash` → `agy` CLI with `--model gemini-3.8-flash-medium`

---

## 3. Workflow & Orchestration Lifecycle

1. **Self-Contained Task Prompt**:
   - Host extracts the prose task, anchors it with the current repo root, target files, acceptance criteria, and points the target CLI to read `AGENTS.md`.
   - Prompt is written to an ephemeral path (e.g. `~/.harnez/runs/prompt-<id>.txt`) to prevent shell-quoting and multi-line escaping errors.

2. **Non-Interactive Background Execution**:
   - Host dispatches the job in the background via CLI:
     ```bash
     codex -a never -s danger-full-access exec -m gpt-5.6-sol "$(cat ~/.harnez/runs/prompt-<id>.txt)"
     ```
     *(Or via the thin wrapper `harnez agent run --tool codex -m gpt-5.6-sol --prompt-file ...` once implemented)*.
   - Host remains responsive, reporting the dispatch handle immediately without blocking.

3. **Independent Verification Gate**:
   - When the process finishes, the host inspects `git status` and `git diff`.
   - Host runs independent repo tests (`go test ./...` / `make check`) rather than trusting the subagent's self-reported success.

4. **Telemetry & Teardown**:
   - Logs `call_type='agent_dispatch'` telemetry (tool, model, duration, exit code).
   - Reports concise summary and diff back to the user.

---

## 4. Implementation Plan

1. **Skill Template (`commands/harnez-agent.md`)**:
   - Define `/harnez-agent` skill accepting `<tool>:<model>` and `task:<prose>`.
   - Document the execution steps, backgrounding rules, and verification checklist.
2. **Register in `config.yaml`**:
   - Add `harnez-agent` to `commands:` and `skills:` lists in `config.yaml`.
3. **Model Resolution / CLI Adapter**:
   - Implement `internal/agent` adapter logic to parse tool:model strings and construct clean command invocations.
4. **Smoke Test**:
   - Run a real MVP trial: `agy` orchestrator dispatching `/harnez-agent codex:sol task:...` and verifying output.
