# 417 — harnez subagent MVP: unified CLI for spawning and resuming Codex and Claude developer subagents

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics / Infrastructure
**Related**: #416, #303, #304, #393, `docs/practices/AgenticLoop.md`

---

## 1. Problem Statement & Motivation

During autonomous developer workflows and multi-phase sprints, orchestrating agents need to spawn, monitor, and resume specialized coding subagents (such as OpenAI Codex or Claude Code) to execute discrete tasks or milestones.

Currently, invoking these agents non-interactively requires remembering and stitching together complex, harness-specific CLI flags, approval policies, and sandbox modes. For example, running Codex non-interactively without hangs requires:
```bash
# Spawning a persistent, tool-capable Codex session:
codex -a never -s danger-full-access exec -C <working_dir> "<prompt>"

# Resuming that session with full context memory:
codex -a never -s danger-full-access exec resume <session_id> "<next_prompt>"
```

Without a unified Harnez abstraction:
1. Orchestrators must memorize low-level flags (`-a never`, `-s danger-full-access`, `--skip-git-repo-check`, `--ephemeral`, `session id` parsing).
2. Resuming and reusing developer sessions is error-prone and requires custom regex parsing from stdout.
3. Adding additional subagent backends (e.g., Claude Code, AGY) creates redundant glue code across scripts and skills.

An MVP `harnez subagent` command group provides a clean, unified interface for spawning, resuming, and managing developer subagents.

---

## 2. Technical Discoveries: OpenAI Codex CLI Invocation & Resumption

Through live smoke testing and empirical probe validation:

1. **Non-Interactive Tool Execution**:
   - Running `codex exec` without flags defaults to interactive confirmation prompts for shell commands and file edits, which causes automated headless callers to stall or cancel.
   - `-a never` (`--ask-for-approval never`): Configures the model never to ask for user approval; errors are directly returned to the model.
   - `-s danger-full-access` (`--sandbox danger-full-access`): Enables tool execution without restrictive sandbox barriers.
   - `-C <DIR>`: Specifies the working root directory.
2. **Session Persistence & Context Retention**:
   - Omitting `--ephemeral` writes session logs to `~/.codex/sessions/` and emits a header containing:
     ```
     session id: 01a0b369-f462-7741-8ced-8d97fd2f8bac
     ```
   - Running `codex -a never -s danger-full-access exec resume <session_id> "<prompt>"` successfully resumes the conversation thread, retaining full memory of prior variables, edits, and reasoning across sequential turns.
3. **Claude Code Analog**:
   - Initial dispatch: `claude -p "<prompt>" --output-format json` (or interactive session resumption via session IDs in `~/.claude/projects/`).

---

## 3. Subagent MVP Command Surface

The initial MVP will support a minimal, focused set of verbs targeting `codex` (default) and `claude`:

```bash
# 1. Spawn / Run a subagent session (prints session ID & response text)
harnez subagent run [--agent codex|claude] [-d <dir>] "<prompt>"

# 2. Resume an existing session by ID
harnez subagent resume <session_id> "<next_prompt>"

# 3. List recent subagent sessions
harnez subagent list [--agent codex|claude]
```

Output formats:
- Clean, human-readable terminal output by default.
- `--json` emitting structured metadata (`session_id`, `agent`, `status`, `tokens_used`, `duration_ms`, `response`).

---

## 4. Measurable Development Milestones

### Milestone 1: Subagent Runner Engine (`internal/subagent/`)
- **Goal**: Implement Go backend driver for spawning and resuming Codex sessions non-interactively.
- **Scope**:
  - `internal/subagent/driver.go`: Define `Driver` interface with `Run(ctx, req RunRequest) (RunResult, error)` and `Resume(ctx, sessionID string, prompt string) (RunResult, error)`.
  - `internal/subagent/codex.go`: Implement Codex driver executing `codex -a never -s danger-full-access exec` and `exec resume`. Extract `session_id`, tokens used, and response output.
- **Verification Target**:
  - `go test -v ./internal/subagent/...` with mock CLI outputs and an isolated integration test verifying output parsing.

---

### Milestone 2: `harnez subagent` CLI Verbs
- **Goal**: Provision the `harnez subagent` command tree in `cmd/harnez/subagent.go`.
- **Scope**:
  - `harnez subagent run [-d <dir>] [--agent <name>] "<prompt>"`
  - `harnez subagent resume <session_id> "<prompt>"`
  - `harnez subagent list` (inspecting recent sessions from `~/.codex/` and Harnez telemetry)
- **Verification Target**:
  - `go test -v ./cmd/harnez/subagent_test.go` verifying flag parsing, execution forwarding, and `--json` format.

---

### Milestone 3: Telemetry Integration & Smoke Verification
- **Goal**: Record subagent dispatches into `cli_invocations` and `tool_calls`, and verify live execution.
- **Scope**:
  - Log `harnez subagent` runs in `~/.harnez/tool_catalog.sqlite`.
  - Perform live smoke test spawning a Codex session via `harnez subagent run` and resuming via `harnez subagent resume`.
- **Verification Target**:
  - Real end-to-end smoke test passes without manual flags.
