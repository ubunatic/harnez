# Research Study: Quota-1 Guardrails for LLM Agent Loops

**Status**: Implemented
**Date**: 2026-09-16
**Related Issue**: #364

## 1. Introduction & Motivation

As autonomous coding loops and TDD workflows become more prevalent, agent models frequently struggle with test execution discipline. Common failure modes include running multiple tests in a single tool step, endlessly re-running identical test suites without intermediate code modifications, and spamming redundant checks when confused. 

"Quota-1" is an emerging pattern designed to constrain this behavior by enforcing a strict **one test execution per step/turn** limit. This research document analyzes the semantics, architectural mechanisms, and scaffolding strategies required to introduce Quota-1 guardrails via `harnez init`.

## 2. Quota-1 Concept & Semantics

### 2.1 Definition of Quota-1
The Quota-1 approach allows the agent to execute a test suite exactly once per logical unit of work (step or turn). If the agent attempts a second test execution within the same window, the system intercepts the request and blocks it, returning immediate feedback to the agent.

### 2.2 Turn & Step Boundaries
A "step" or "turn" boundary corresponds to the discrete action-observation loop of the agent harness:
- **Claude Code**: A single assistant message emitting tool calls before yielding for user or system responses.
- **Google Antigravity (AGY)**: A planner response step generating command invocations.
- **Codex**: A similar action cycle.

### 2.3 Command Classification (Test vs. Non-Test)
Not all shell executions are subject to the quota. We differentiate:
- **Exempt (Unlimited)**: Syntax checks, static analysis, linters (e.g., `go vet`, `golangci-lint`), and pure compilation/builds (e.g., `go build`). These are fast, deterministic, and safe to loop.
- **Quota-Restricted**: Heavy test executions and canary scripts (e.g., `go test`, `make smoke`, `pytest`). 

### 2.4 Quota State Tracking & Reset Conditions
State tracking requires persistent context across shell invocations within a step. The quota resets when:
1. **File Modification**: An edit or write operation is detected in the workspace (indicating the agent changed logic).
2. **Turn Increment**: The step boundary is crossed (the agent yielded to the user or completed a tool feedback loop).
3. **Explicit Reset**: A dedicated bypass flag or environment variable is passed.

## 3. Enforcement Architectures

Enforcement must occur at the interception layer where the agent invokes shell commands. Based on multi-harness hook architectures (`docs/HookRewritePattern.md`), the options are:

### 3.1 Claude Code: PreToolUse Hooks
- **Mechanism**: The `PreToolUse` hook intercepts `Bash` tool calls.
- **Flow**: The hook inspects the command. If it matches a test pattern, it checks the quota state. If exceeded, it rewrites the command to an `echo` statement that returns a strict error message (e.g., "Quota exceeded: You must edit code before running tests again").
- **Pros**: Intercepts before execution, native to the harness.
- **Cons**: Requires hook registration, state must be tracked via temp files or IPC since the hook process is ephemeral.

### 3.2 Google Antigravity: PATH Shims
- **Mechanism**: The existing guarded `bash` PATH shim (`~/.harnez/shims/bash`) wraps the `run_command` tool.
- **Flow**: The shim checks if the command arguments resemble test execution. It reads a local quota state file. If exceeded, it exits immediately with a descriptive error.
- **Pros**: Transparent to the UI, avoids leaking wrapper plumbing into traces.
- **Cons**: Slightly harder to isolate step boundaries natively compared to API hooks.

### 3.3 Makefile Wrappers (Project-Local)
- **Mechanism**: `make test` acts as the enforcer.
- **Flow**: The Makefile target checks a local lockfile.
- **Pros**: Independent of the agent harness, applies universally.
- **Cons**: Agents might bypass `make` and run `go test` directly unless specifically instructed via `AGENTS.md`.

## 4. Scaffolding via `harnez init`

`harnez init` is responsible for project-local scaffolding (`docs/CLIDesign.md`). To provide Quota-1 guardrails:

### 4.1 Opt-in vs. Default
Given the restrictive nature of Quota-1, it should be an **opt-in flag** during initialization: `harnez init --quota-1`.

### 4.2 Scaffolded Artifacts
1. **`AGENTS.md` Rules**: Inject rules directing agents to rely on the enforced test commands and explaining the Quota-1 constraint so they expect the blocks.
2. **Local Hook Definitions**: For project-local hooks, create a `.claude/hooks.json` or `.harnez/hooks/` entry that points to a project-local shim.
3. **Makefile Targets**: Inject `test-quota` wrappers into the `Makefile`.

### 4.3 Bypass Mechanisms
- **Human Developers**: Bypassed automatically by checking for non-agent TTY, or via `QUOTA_BYPASS=1`.
- **CI Runs**: Bypassed implicitly as CI environments lack agent tracking IDs.
- **Review Agents**: Phase 3 reviewer subagents can pass an override flag if comprehensive test runs are required.

## 5. Ergonomics & Failure Modes

- **Blocking vs. Warning**: Blocking with clear feedback is required. Warnings are often ignored by LLMs, leading to continued token burn on redundant tests.
- **State Stagnation**: If the turn reset detection fails, the agent may be permanently locked out of testing. The state tracker must reliably reset on ANY file write or turn transition.
- **Bypass Overuse**: Agents might attempt to learn the bypass flags and use them maliciously to escape the quota. The prompt must strictly forbid unauthorized use of the bypass flag.

## 6. Concrete Recommendations

1. **Implement Quota Tracking via Shell Wrappers**: Leverage the `harnez exec` pattern to wrap test invocations. The wrapper writes a timestamped lockfile to `.git/harnez/quota_1.state`.
2. **File Mod Reset**: Introduce an fsnotify watcher or mtime checker in the wrapper that compares the timestamp of source files against the test lockfile. If source files are newer, the quota is implicitly reset.
3. **Add `harnez init --quota-1`**: Scaffold a Makefile target `test-q1` that invokes the wrapper, and update `AGENTS.local.md` to instruct agents to use `make test-q1`.
4. **Agent Prompts**: Ensure `AGENTS.md` explicitly teaches the agent: "You are under Quota-1 rules. If a test fails, you MUST modify code before re-running. Do not attempt to bypass."

## 7. Implementation Architecture & Shipped Components

The recommendations above have been implemented across three core layers:

### 7.1 State Tracker (`internal/quota1`)
- **State File**: Stored in `.git/harnez/quota_1.state` (or `.harnez/quota_1.state` in non-git directories / worktrees).
- **Check Logic (`CheckAndRecord`)**:
  1. Checks `QUOTA_BYPASS=1` or `HARNEZ_QUOTA_BYPASS=1`; if active, records run and permits execution.
  2. If the state file does not exist, records initial run and permits execution.
  3. If the state file exists, walks the repository tree comparing source file `ModTime` with the state timestamp, excluding `.git`, `.harnez`, `vendor`, `node_modules`, `dist`, `build`, and temporary/backup files.
  4. If source files were modified, updates timestamp and permits execution.
  5. If no source files were modified, blocks execution with a descriptive error message without updating the state timestamp.

### 7.2 CLI Execution Interceptor (`cmd/harnez/exec.go`)
- Added `--quota-1` flag to `harnez exec` and `HARNEZ_QUOTA_1=1` environment variable support.
- Invokes `quota1.CheckAndRecord` prior to subprocess creation.
- When blocked, writes the explanation to `stderr` and exits immediately with status code `1` without spawning the subprocess or logging deceptive telemetry rows.

### 7.3 Scaffolding & Conventions (`cmd/harnez/init.go`, `internal/claude/init.go`)
- Added `--quota-1` flag to `harnez init`.
- Injects managed `Quota-1 Guardrails` section into `AGENTS.md` via `applySectionMD`, establishing expectations for agent models.
- Reconciles `test-q1` Makefile target (`test-q1: 🤖 # run tests under Quota-1 enforcement\n\t⚙ --quota-1 -- $(MAKE) test`).

---
*End of Document*

