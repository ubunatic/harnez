# 364 — Research Quota-1 (single test run per step) guardrails for LLM loops via harnez init

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [docs/practices/AgenticLoop.md](../docs/practices/AgenticLoop.md), [docs/HookRewritePattern.md](../docs/HookRewritePattern.md), [issues/118-harnez-exec-shell-interceptor.md](118-harnez-exec-shell-interceptor.md), [issues/177-lean-post-edit-build-check-for-control-flow-edits.md](177-lean-post-edit-build-check-for-control-flow-edits.md)

---

## 1. Problem & Motivation

In autonomous and iterative LLM agent loops (TDD cycles, bug-fixing, refactoring), models frequently exhibit test-thrashing anti-patterns:
1. **Multi-Test Spamming**: Running multiple test commands consecutively within a single turn without intermediate code edits or reasoning.
2. **Broad Test Suite Spam**: Re-running entire test suites (e.g. `go test ./...` or `pytest`) repeatedly instead of executing a single, targeted unit test.
3. **Flakiness & Context Bloat**: Consuming thousands of tokens on repetitive test outputs in a single turn without forming a fresh hypothesis.

The **Quota-1 pattern** enforces a strict execution budget during development loops: **an LLM is permitted at most ONE test run per turn/step**. 

If the agent attempts to execute a second test within the same step (or before modifying code), the harness intercepts the execution, blocks the redundant run, and reminds the agent to inspect the existing test results, formulate a hypothesis, and make code edits before re-testing.

This ticket researches how `harnez init` can scaffold and enforce Quota-1 guardrails across client repositories.

---

## 2. Quota-1 Guardrail Architecture & Mechanics

### 2.1 Enforcement Points
- **PreToolUse Hook Interception**:
  A `PreToolUse` hook (extending `harnez exec hook` or a project-level test wrapper) monitors tool invocations matching test runners (`go test`, `pytest`, `cargo test`, `npm test`, `make test`, `ctest`, etc.).
- **Step / Turn State Accounting**:
  - Ephemeral state (in `internal/sessionstate/` or `/tmp/.harnez-step-*` / environment variable) records test executions within the current agent turn.
  - Test quota resets to 1 whenever a file edit (write/patch) occurs or a new turn begins.
- **Quota Exceeded Action**:
  When a second test execution is attempted without an intervening code modification:
  - The hook rejects the invocation with an actionable diagnostic message:
    `"Quota-1 Guardrail: Only 1 test run permitted per step. Analyze the prior failure, modify code, and run a targeted test."`
  - Prevents runaway token consumption and halts infinite test-retry loops.

### 2.2 Promoting Targeted Test Execution
- Quota-1 can be coupled with targeted test filters (e.g. requiring `-run <TestName>` or specific test file targeting during development loops, reserving full-suite runs for Phase 3 review gates).

---

## 3. Integration & Scaffolding via `harnez init`

`harnez init` configures project-level agent environments. Potential scaffolding mechanisms include:

1. **`AGENTS.md` / `AgenticLoop.md` Invariant**:
   - Add a canonical invariant in Phase 2 (Sequential Development & TDD): *Invariant: Quota-1 Test Discipline — one hypothesis, one targeted test execution per turn*.
2. **Project Hook Scaffolding**:
   - Provisioning PreToolUse test interceptor hooks in `settings.json` (Claude Code), `hooks.json` (AGY), or shell wrappers (Codex).
3. **Makefile Guardrails**:
   - Scaffolding test runner targets in project `Makefile`s (e.g. `make test-unit`, `make test-single`) that integrate with the Quota-1 state tracker.
4. **Language-Specific Guidance in `docs/lang/`**:
   - Updating `docs/lang/Go.md`, `Bash.md`, `Rust.md`, `Cpp.md`, etc., with explicit single-test targeting syntax (e.g. `go test -run TestX ./pkg`).

---

## 4. Research & Design Questions

1. **Turn Boundary & File Edit Detection**:
   - How can the hook reliably differentiate between a new turn vs. multiple tool calls within the same turn across Claude Code, Google Antigravity, and Codex?
   - Can file modification timestamps (`fsutil`) or post-edit hooks reset the turn quota cleanly?
2. **Failure vs. Build/Syntax Checks**:
   - Distinguishing quick compiler/syntax checks (e.g. `go build`, `tsc --noEmit`) from heavy test runner executions (`go test ./...`).
3. **Opt-in vs. Default-on Scaffolding**:
   - Should `harnez init` enable Quota-1 test guardrails by default, or provide a `--quota-1` / `--strict-tdd` opt-in flag?
4. **Bypass Mechanism**:
   - How can human developers or Phase 3 review subagents legitimately run full test suites without tripping the single-test limiter?

---

## 5. Acceptance Criteria

- [ ] Analyze turn/step boundary detection mechanisms across supported agent harnesses.
- [ ] Prototype a PreToolUse hook interceptor that tracks test runner invocations and enforces a 1-test-per-step limit.
- [ ] Draft the Quota-1 TDD invariant for `docs/practices/AgenticLoop.md`.
- [ ] Design the `harnez init` scaffolding integration (hooks, Makefile recipes, `AGENTS.md` rules).
- [ ] Implement a canary test verifying that a 2nd consecutive test execution is rejected with a clear remediation message.
