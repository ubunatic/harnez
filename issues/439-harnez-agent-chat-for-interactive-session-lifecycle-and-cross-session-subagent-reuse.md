# 439 — harnez agent chat for interactive session lifecycle and cross-session subagent reuse

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agent Orchestration / Interactive CLI
**Related**: #342, #438

---

## 1. Problem & Motivation

Currently, `harnez agent start` dispatches an agent in batch / headless execution mode, returning only once the prompt turn completes. Users and host orchestrators lack a way to:
1. Start an **interactive chat session** (`harnez agent chat <provider:model>`) using a specified agent environment.
2. Control and inspect all active sessions from a unified registry, sending commands or follow-up prompts into sessions that a human user is concurrently interacting with.
3. Automatically register each interactive or headless session with a friendly, memorable identifier (e.g. random short name like `swift-falcon` or a user-specified `--name`).
4. Allow interactive sessions to seamlessly double as reusable subagents in other orchestrations (via `harnez agent resume <name-or-id> "<prompt>"` or `harnez agent compact`).

---

## 2. /goal & Acceptance Criteria

### /goal
Introduce `harnez agent chat` to launch interactive sessions with registered short names, enabling unified control, bi-directional interaction (user and orchestrator prompts), and seamless reuse of interactive sessions as subagents.

### Acceptance Criteria
1. **Interactive Chat Command**:
   - `harnez agent chat <provider:model> [--name <name>] [-d <dir>]`: Launches an interactive terminal chat session with the target agent backend (e.g. `codex`, `claude`, `agy`, `local`).
   - If `--name` is omitted, automatically assigns and registers a memorable random short name (e.g. `calm-otter`, `bold-fox`).
2. **Session Registry & Discovery**:
   - The session is registered in the session store (`~/.harnez/agents/`) with its short name, PID, provider, model, and active status.
   - `harnez agent list` and `harnez agent status` display the short name alongside the session ID.
3. **Dual Interaction & Subagent Reuse**:
   - Both human users (via interactive terminal) and host orchestrators (via `harnez agent resume <name-or-id> "<prompt>"`) can dispatch prompts into the session.
   - Interactive sessions can be paused, resumed, compacted, or handed off as developer/reviewer subagents in sprint workflows.
4. **Clean Detach & Reattach**:
   - Supports cleanly detaching from an interactive session and reattaching later (`harnez agent chat attach <name-or-id>`).
5. **Unit & Integration Tests**:
   - Tests verify name generation, registration in session store, CLI argument parsing, and state transitions.
