# 634 — harnez decide: hooks & rule enforcer, tool guardrails, and Jev-as-judge

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Normal
**Category**: Feature
**Related**: issues/633, issues/635, internal/claude/apply.go, internal/agy/hooks.go, internal/codex/hooks.go, docs/practices/AgenticLoop.md

---

## 1. Problem & Motivation
Agent workflows across Claude Code, Codex, AGY (Antigravity), and Pi can perform unintended operations:
- Executing destructive or irreversible terminal commands (e.g. `rm -rf`, force pushes, unverified package installations).
- Violating repository or file-level guidelines (e.g. modifying read-only/generated files, introducing forbidden dependencies, breaking style/API rules).
- Defaulting to slow, lossy LLM summarization during context compaction instead of fast verbatim pruning.
- Running long, expensive review passes on trivial changes when a quick pre-filter would suffice.

Currently, safety checks either rely on brittle regexes or heavy LLM invocations.
Using the `harnez decide` engine (issue 633), we can implement high-speed (~300ms), low-cost guardrails and lifecycle triggers across all supported agent hook environments.

## 2. Technical Specification / Findings
> Note 2026-09-29: `harnez decide` exists (issue 633, `docs/Decide.md`). Backends are chosen
> with `--backend`; read `--model jev` here as `--backend jev`. Gates must treat exit 2 as unknown.

### Cross-Agent Hook Surface Analysis
Harnez supports multiple host agents, each with its own hook schema and lifecycle events:

| Lifecycle Phase | Claude Code (`settings.json` / plugin) | AGY (`~/.gemini/config/hooks.json`) | Codex (`config.toml` `[[hooks.*]]`) | Pi Agent |
|---|---|---|---|---|
| **Pre-Tool Execution** | `PreToolUse` (matchers: `Bash`, `Edit`, `Write`) | `PreToolUse` (matcher: `run_command`, `write_to_file`, `replace_file_content`) | `PreToolUse` (matcher: `Bash`, file ops) | `tool_call` filter / middleware |
| **Post-Tool Execution** | `PostToolUse` | `PostToolUse` | `PostToolUse` | `tool_result` filter |
| **Context Compaction** | `PreCompact` / `Compaction` hook (or plugin replacement) | Transcript compaction event / pipeline | Session prune trigger | Turn-end compaction check |
| **User Prompt / Turn Start** | `UserPromptSubmit` | `SessionStart` / prompt hook | Session prompt filter | Prompt middleware |

---

### Core Hook Integration Points for `harnez decide`

#### 1. Pre-Compact Hook (Verbatim Compaction)
- **Trigger**: When the agent's context window exceeds threshold or compaction is triggered.
- **Action**: Intercept the compaction request and invoke `harnez compact --model jev` (or `--model lmcoder`).
- **Mechanism**:
  - Claude Code: Hook intercepting compaction / plugin hook replacing summarizer.
  - Passes conversation history JSON to `harnez compact`.
  - Stale tool uses/results are pruned or truncated while conversation text is kept verbatim.
  - Completes in <1s rather than 20–30s.

#### 2. PreToolUse: File Edit Rule Enforcer
- **Trigger**: Before executing `Edit`, `Write`, `replace_file_content`, or `write_to_file`.
- **Payload received**: Target file path, proposed patch/content.
- **Action**:
  - Harnez extracts applicable rules from `AGENTS.md` and `.harnez/rules/` for that file path.
  - Queries `harnez decide`:
    ```json
    {
      "rule_violation": {
        "type": "noul",
        "instructions": "Does this edit violate any of the stated rules for `target_file`?",
        "criteria": {
          "true": "Modifies generated files directly, breaks architectural layering, bypasses canary/test rules",
          "false": "Standard compliant code change"
        }
      },
      "violated_rule": {
        "type": "choice",
        "instructions": "Which rule is violated?",
        "criteria": { ...rules_map... }
      }
    }
    ```
  - **Threshold Policy**: If `rule_violation.noul >= 0.90` (or configured threshold), the hook aborts tool execution with exit code 2 and returns stderr feedback:
    > "Edit blocked: Jev rule enforcer detected violation of rule [<rule_name>] (confidence 0.94): <explanation>."

#### 3. PreToolUse: Terminal / Command Guardrail
- **Trigger**: Before executing `Bash` (Claude/Codex) or `run_command` (AGY).
- **Payload received**: Command string, working directory.
- **Action**:
  - Intercepted by `harnez exec` / hook handler.
  - Checks for irreversibility, destructive actions, or quota bypasses.
  - `irreversible.noul > 0.85` -> blocks command.
  - Uncertain confidence (`< 0.50`) -> prompts operator.

#### 4. Pre-Turn / Skill Picker Hook
- **Trigger**: `UserPromptSubmit` / turn start.
- **Action**:
  - Evaluates user prompt against available external and built-in skills.
  - Selects the matching skill (or none) and injects a light prompt hint without blowing up the context window.

#### 5. Review & Pre-Commit Gates (Sprint Phase 3)
- **Trigger**: Pre-commit hook or `/sprint` review phase.
- **Action**:
  - **Jev as a Judge**: Evaluates specifications vs tests. If access rules lack tests, halts commit and lists required test scenarios.
  - **Cheap First-Pass Review**: Evaluates diff risk score; bypasses heavy subagent review for trivial changes.

## 3. Implementation & Verification Plan
1. **Hook Handlers in Go**:
   - Implement `cmd/harnez/hook.go` subcommands:
     - `harnez hook pre-edit`
     - `harnez hook pre-exec`
     - `harnez hook pre-compact`
2. **Agent Hook Injectors**:
   - Wire handlers into `internal/claude/apply.go` (`~/.claude/settings.json`), `internal/agy/hooks.go` (`~/.gemini/config/hooks.json`), and `internal/codex/hooks.go` (`~/.codex/config.toml`).
3. **Configuration**:
   - Enable/disable toggles in `~/.harnez/config.yaml` under `hooks.decide.*` (e.g. `rule_enforcer: true`, `threshold: 0.90`).
4. **Verification**:
   - Automated tests with simulated tool payloads for Claude, Codex, and AGY.
   - Verify that an edit violating rules is blocked with exact rule feedback.
   - Verify that `harnez compact` cleanly replaces the built-in compaction flow.
