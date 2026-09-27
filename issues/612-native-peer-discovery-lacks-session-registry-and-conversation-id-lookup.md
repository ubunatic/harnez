# 612 — Native peer discovery lacks session registry and conversation ID lookup

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**:

---

## 1. Problem & Motivation

When coordinating across sessions via `/peer-assistant` or native messaging tools (e.g. `send_message`), an agent cannot discover or message native peer sessions running in other workspaces (e.g. telling session "loom" in `/home/uwe/projects/loom` that "cati" is back online):

- `send_message` requires a specific `Recipient` conversation ID (UUID). Attempting to pass logical names (e.g. `Recipient: "loom"`) returns `recipient "loom" not found`.
- `manage_subagents list` only lists direct subagents spawned by the current agent process, returning `[]` for peer host sessions.
- `harnez agent list` / `harnez_list_agents` only tracks agents started through the Harnez agent lifecycle manager, omitting native interactive host sessions (e.g. `agy` instances running in `/home/uwe/projects/loom`).
- To locate the target session, the agent had to fall back to raw process inspection (`ps aux | grep agy`, `pwdx`), inspecting open file descriptors (`lsof`), and reading `~/.gemini/antigravity-cli/presence/*.lock` to correlate the working directory `/home/uwe/projects/loom` with its conversation ID `4fbc8aa5-a04d-4d5d-b2e5-21a66127deea`.

Agents need a reliable discovery mechanism or alias resolution for active native peer sessions across workspaces.

## 2. Technical Specification / Findings

### Current Behavior & Failure Modes
1. Calling `send_message` with `Recipient: "loom"`:
   ```
   Encountered error in tool execution: recipient "loom" not found
   ```
2. Session identification details:
   - Presence locks are stored under `~/.gemini/antigravity-cli/presence/<conversation-id>.lock` (and conversation sqlite databases under `conversations/<id>.db`).
   - Process arguments or PID working directory (`/proc/<pid>/cwd`) link the PID to the project directory, and the process holds open handles to the corresponding conversation DB and presence lock.
   - No Harnez CLI command or MCP tool maps running native sessions / workspaces to their conversation IDs.

### Requirements
- Provide a discovery command / tool (e.g., `harnez peer list` or integrating native session discovery into `harnez find` / `harnez agent list`).
- Report active native sessions with their conversation ID, provider/harness type (`agy`, `claude`, `codex`), working directory, and PID / lock status.
- Allow resolving peer session aliases/directory names to conversation IDs for peer communication.

## 3. Implementation & Verification Plan

### Implementation
1. Add native session discovery to Harnez (e.g. reading presence directories and `/proc` or platform-appropriate session registries).
2. Support mapping workspace basename / project path to active conversation ID.
3. Expose the discovery via CLI (`harnez peers` or `harnez agent list --native`) and ensure skills/guidelines document how peer assistants resolve target recipients.

### Verification
- Launch a test session in a separate workspace.
- Run the discovery tool from another workspace and verify it accurately reports the conversation ID, workspace path, and status.
- Verify `send_message` can be addressed using the discovered ID.
