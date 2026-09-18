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

Crucially, **subagent invocation must not be a blind tool call**: the orchestrator requires explicit, real-time awareness of the subagent's **cumulative token size and turn token consumption**. Without structured token telemetry returned by the subagent runner, the orchestrator cannot make informed decisions about:
- When to safely **resume** an existing developer session (leveraging prefix KV-caching for sequential milestone work), versus
- When to **start fresh** to avoid paying the escalating context tax of an over-bloated conversation history.

An MVP `harnez subagent` command group provides a clean, unified interface for spawning, resuming, and managing developer subagents with first-class token metrics.

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
   - Tail of Codex run emits exact cumulative token metrics:
     ```
     tokens used
     15.634
     ```
3. **Claude Code Analog**:
   - Initial dispatch: `claude -p "<prompt>" --output-format json` (exposes `usage.iterations` and total token counts).

---

## 3. Subagent MVP Command Surface & Informed Output Contract

The initial MVP will support a minimal, focused set of verbs targeting `codex` (default) and `claude`:

```bash
# 1. Spawn / Run a subagent session (prints session ID, token metrics, and response text)
harnez subagent run [--agent codex|claude] [-d <dir>] "<prompt>"

# 2. Resume an existing session by ID
harnez subagent resume <session_id> "<next_prompt>"

# 3. List recent subagent sessions with token usage totals
harnez subagent list [--agent codex|claude]
```

### Informed Output Contract:
Every subagent execution returns structured token telemetry directly to the orchestrator:
- **Terminal Output**: Summary header with `session_id`, `agent`, `tokens_used` (cumulative), `turn_tokens`, and `duration_ms`, followed by response text.
- **`--json` Payload**:
  ```json
  {
    "session_id": "01a0b369-f462-7741-8ced-8d97fd2f8bac",
    "agent": "codex",
    "status": "completed",
    "tokens_cumulative": 15634,
    "tokens_turn": 3647,
    "duration_ms": 7240,
    "response": "SUCCESS: All tests passing"
  }
  ```

This allows orchestrating scripts and agent loops to inspect `tokens_cumulative` and enforce context budget limits (e.g. threshold $>50\text{k}$ tokens triggers a fresh subagent spawn instead of further resumption).

---

## 4. Measurable Development Milestones

### Milestone 1: Subagent Runner Engine (`internal/subagent/`)
- **Goal**: Implement Go backend driver for spawning and resuming Codex sessions non-interactively with exact token parsing.
- **Scope**:
  - `internal/subagent/driver.go`: Define `Driver` interface with `Run(ctx, req RunRequest) (RunResult, error)` and `Resume(ctx, sessionID string, prompt string) (RunResult, error)`.
  - `internal/subagent/codex.go`: Implement Codex driver executing `codex -a never -s danger-full-access exec` and `exec resume`. Extract `session_id`, `tokens_used`, and response text from stdout/JSON streams.
- **Verification Target**:
  - `go test -v ./internal/subagent/...` with mock CLI outputs and an isolated integration test verifying token extraction and output parsing.

---

### Milestone 2: `harnez subagent` CLI Verbs
- **Goal**: Provision the `harnez subagent` command tree in `cmd/harnez/subagent.go`.
- **Scope**:
  - `harnez subagent run [-d <dir>] [--agent <name>] "<prompt>"`
  - `harnez subagent resume <session_id> "<prompt>"`
  - `harnez subagent list` (inspecting recent sessions from `~/.codex/` and Harnez telemetry)
  - Ensure `--json` emits `tokens_cumulative`, `tokens_turn`, `duration_ms`, and `session_id`.
- **Verification Target**:
  - `go test -v ./cmd/harnez/subagent_test.go` verifying flag parsing, execution forwarding, and `--json` format.

---

### Milestone 3: Telemetry Integration & Smoke Verification
- **Goal**: Record subagent dispatches into `cli_invocations` and `tool_calls`, and verify live execution.
- **Scope**:
  - Log `harnez subagent` runs in `~/.harnez/tool_catalog.sqlite`.
  - Perform live smoke test spawning a Codex session via `harnez subagent run` and resuming via `harnez subagent resume`.
- **Verification Target**:
  - Real end-to-end smoke test passes without manual flags and confirms token metrics are populated.

---

## 5. Research & Follow-Up Analytics Scope (Next Ticket)

Once this MVP is operational and a series of subagents have been spawned in real sprints:

1. **Telemetry Analysis**: Inspect `harnez stats` and `tool_catalog.sqlite` across live subagent sessions to examine token growth rates, turn counts, and cost-per-milestone trends.
2. **Follow-Up Analytics Ticket (To be filed by MVP implementer)**:
   - File a follow-up ticket for **Subagent Session Analytics & Automated Reuse Policies**.
   - Design features for:
     - Tracking **token velocity** (tokens consumed per milestone).
     - Measuring **KV-cache reuse efficiency** across resumed turns.
     - Automated **"fresh spawn vs resume" advisory recommendation** in `harnez subagent` based on context size and model pricing thresholds.
