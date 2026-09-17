# Local LLM Serving, Prompting & Sandboxed Agent Execution with lmcoder

Use `lmcoder` to manage local LLM serving, execute fast interactive prompts with automatic session continuity, and run sandboxed coding agents inside rootless Podman.

---

## When Agents Should Use lmcoder

| Capability Level | Command | Best For | Overhead / Latency |
|---|---|---|---|
| **L1: Fast Prompt** | `lmcoder prompt "<query>"` | Quick Q&A, code reviews, formatting, text cleanup, unit tests | Instant (< 1s TTFT), streaming SSE |
| **L2: Sandboxed Agent** | `lmcoder agent --agent <name>` | Multi-step coding tasks, full workspace refactoring, sandboxed execution | High (Container startup + agent loop) |
| **Model Lifecycle** | `lmcoder load <model>` | Dynamically switching models on AMD APU / GPU VRAM+GTT | Fast (~1-3s model swap) |

---

## Key Commands & Usage

### 1. Fast Prompting & Session Continuity (`lmcoder prompt`)
Send prompts directly to local models or cloud OpenAI Codex. Automatically resumes conversation history tied to the active TTY terminal session.

```bash
# Basic prompt to local active model (streaming output)
lmcoder prompt "Explain the error handling in main.go"

# Inject file context directly
lmcoder prompt -f internal/llamahost/session.go "How is the session ID computed?"
# or using inline @file syntax
lmcoder prompt "Refactor this function: @cmd/lmcoder/load.go"

# Pipe input via stdin
cat logs/server.log | lmcoder prompt "Extract error stack traces"

# Target cloud OpenAI Codex with session resumption
lmcoder prompt --host codex "Review current git diff"

# Start fresh session (ignore cached TTY history)
lmcoder prompt --fresh "Ask a clean question"
```

### 2. Sandboxed Autonomous Agent (`lmcoder agent`)
Run autonomous coding agents in an isolated, hardened Podman container with tool access:

```bash
# Run prime-agent against local llama-server
lmcoder agent --agent prime-agent "Implement the missing unit tests for load.go"

# Run Pi coding agent with read-only volume mounting
lmcoder agent --agent pi "Analyze codebase architecture"
```

Supported agents: `prime-agent`, `pi`, `opencode`, `codex`, `claude`.

### 3. Model Lifecycle (`lmcoder load` / `unload` / `start`)
Manage models dynamically without manually restarting daemons:

```bash
# Start server daemon (defaults to 127.0.0.1:8734 loopback)
lmcoder start

# Load or switch active model with fuzzy matching & VRAM preflight
lmcoder load qwen3-4b
lmcoder load smollm3-3b
lmcoder load 0.5b

# Unload active model to free GPU/GTT memory
lmcoder unload
```
