# 405 — Implement strict PreToolUse hook to intercept native view_file and Read tool calls with harnez read

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Hooks & PreToolUse / Reading Discipline
**Related**: #403, #193, #195, #395, #400, #401

---

## 1. Problem Statement & Motivation

Despite explicit instructions codified in repository guidelines (`AGENTS.md`, `config.yaml`, `AgenticLoop.md` Invariant 6: *Reading & Context Discipline*), LLM agents continually fall back to calling built-in IDE read tools (`view_file`, `View`, `ReadMultipleFiles`) out of "tool declaration gravity" instead of executing shell-level `harnez read -I` or `harnez read -L <range> -n`.

### Concrete Failure Example:
In our paired development session on 2026-09-17, when preparing a surgical edit for `docs/practices/AgenticLoop.md`, the orchestrating agent instinctively invoked the native IDE `view_file` tool twice to check line ranges for `replace_file_content`, rather than executing `harnez read -L 45:65 -n`.

Prompt-only instructions in `AGENTS.md` and system prompts provide essential guidance but remain leaky under cognitive load. To guarantee zero token bloat and prevent context fatigue, Harnez must mechanically intercept native file-read tool executions using strict PreToolUse hooks and redirect agents to `harnez read`.

---

## 2. Technical Scope & Architecture

### 2.1 PreToolUse Hook Interception (Claude Code, AGY, Codex)
1. **Tool Matcher**:
   - Intercept tool calls matching `view_file`, `View`, `ReadMultipleFiles`, and `read_file`.
2. **Interception Heuristics & Policy**:
   - **Large File Threshold**: If the target file exceeds $\ge 100$ lines (or estimated $\ge 1,000$ tokens), native unconstrained whole-file reads must be blocked or redirected.
   - **Redirect Action**:
     - *Visual Discovery Mode*: Recommend/rewrite to `harnez read -I <file>` (rendering a 1-bit pixel font PNG card and attaching/viewing it).
     - *Precision Editing Mode*: If the agent requires line numbers for a code edit/patch, force or recommend `harnez read -L <range> -n <file>`.
3. **Mechanical Guardrail Feedback**:
   - Emit a structured guidance message when intercepted:
     `"harnez guard: native view_file on large files (>100 lines) violates Reading & Context Discipline. Execute 'harnez read -I <file>' for visual cards or 'harnez read -L <range> -n <file>' for line-bounded editing anchors."`

### 2.2 Integration Across Harnez Subsystems
- **`internal/claude` & `internal/agy`**: Add the PreToolUse hook generator to `harnez apply` to automatically configure `~/.claude/settings.json` and agent tool hook definitions.
- **Canary & Verification**: Extend `scripts/smoke-test.sh` and hook test suites in `cmd/harnez/hook_test.go` to verify that `view_file` calls on large repository files trigger interception.

---

## 3. Acceptance Criteria

- [ ] PreToolUse hook definition implemented in `internal/claude/` (and corresponding cross-harness definitions in `internal/agy/`).
- [ ] Hook intercepts native file-read tools when target files exceed line threshold or when unbounded reads occur.
- [ ] Clear error/redirect feedback guiding the agent to `harnez read -I` or `harnez read -L -n`.
- [ ] Unit tests in `cmd/harnez/` and `internal/claude/` verifying hook generation and matchers.
- [ ] End-to-end verification proving that agents attempting `view_file` on large files are redirected to `harnez read`.
