# 325 — Add /lmcoder skill for local LLM serving, fast prompt streaming, and sandboxed agent execution

**Status**: Closed
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: `commands/lmcoder.md`, `lmcoder:docs/PromptAndSessionArchitecture.md`, `lmcoder:docs/Roadmap.md`

---

## Summary

`lmcoder` provides local LLM lifecycle management (loading, unloading, starting daemons), fast sub-second interactive prompting with automatic TTY-bound session history resumption (`lmcoder prompt`), and sandboxed containerized agent execution inside rootless Podman (`lmcoder agent`).

This issue introduces the `/lmcoder` skill to the `harnez` ecosystem so that autonomous coding agents across all supported harnesses (Gemini/AGY, Claude, Codex, Prime Agent) can discover, invoke, and leverage `lmcoder` for agentic pair-programming workflows.

## Deliverables

1. **Skill Command Specification** (`commands/lmcoder.md`):
   - Outlines when agents should choose **L1: Fast Prompt** (`lmcoder prompt`) vs. **L2: Sandboxed Agent** (`lmcoder agent`) vs. **Model Lifecycle** (`lmcoder load`/`unload`).
   - Details interactive prompt capabilities: `@file` / `-f` context injection, pipe parsing via stdin, cloud OpenAI Codex fallback (`--host codex`), and session continuity.
   - Details sandboxed agent execution: supported agent drivers (`prime-agent`, `pi`, `opencode`, `codex`, `claude`).
   - Details model lifecycle: starting loopback daemons and dynamic model swaps on AMD APU/GPU.
2. **Configuration & Distribution** (`config.yaml`):
   - Registered `lmcoder` under `commands:` and `skills:`.
   - Enabled deployment across all harness skill targets via `harnez apply`.

## Verification

- [x] `commands/lmcoder.md` created with concise capability matrix and command usage examples.
- [x] `config.yaml` updated with `lmcoder` command and skill definitions.
- [x] Ran `harnez apply` to verify deployment to `~/.gemini/skills/lmcoder/`, `~/.claude/skills/lmcoder/`, `~/.codex/skills/lmcoder/`, and `~/.prime/agent/skills/lmcoder/`.
