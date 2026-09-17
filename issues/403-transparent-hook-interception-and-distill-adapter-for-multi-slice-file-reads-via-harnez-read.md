# 403 — Transparent hook interception and distill adapter for multi-slice file reads via harnez read

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Hooks & Distill / Context Optimization
**Related**: [[070-cross-agent-distill-autopipe-hook-agy-codex-pi-opencode]], [[193-research-agy-hook-surface-for-transparent-exec-distill]], [[195-path-shim-wrapper-for-agy-exec-distill-interception]], [[199-research-codex-hook-surface-for-transparent-exec-distill]], [[395-harnez-read-command-with-i-image-flag-for-rendering-and-injecting-whole-files-as-visual-context]], [[400-embed-retro-pixel-font-engine-into-internal-readcard-as-default-across-read-docs-cards-init-and-apply]]

---

## 1. Problem & Motivation

Even when repository instructions (`AGENTS.md`) and subagent prompts instruct agents to use `harnez read -I` or `harnez read -L` for inspecting large files, LLM agents have overwhelming "tool declaration gravity":
- When asked to inspect a file, agents instinctually reach for their native built-in IDE read tools (`view_file`, `View`, `ReadMultipleFiles`) because they are directly registered in the model's tool schema.
- As observed in Issue 401 implementation, a dev subagent did multiple sequential `view_file` slice reads across several files, dumping hundreds of raw text lines into context rather than running `harnez read` via shell execution.

To solve this systematically without relying solely on model prompt adherence, Harnez needs transparent hook interception / distillation for file read operations.

---

## 2. Proposed Architecture & Interception Strategies

1. **Harness Hook Pre-Tool Interception**:
   * For harnesses supporting pre-tool-call hooks (e.g. Claude Code PreToolUse hooks, Codex hooks, or Antigravity extension/proxy hooks):
   * When a file read tool (`view_file`, `View`, `ReadMultipleFiles`) is invoked:
     * Check if the target file exceeds a token/line threshold (e.g. >100 lines) or if it is the 2nd+ read of the same file in the session.
     * Intercept and rewrite the call to execute `harnez read -I <file>` (or `harnez read -n <file>`) or return a dense visual card context attachment.
     * Alternatively, inject a concise high-priority reminder or redirect before execution.

2. **Cross-Agent Distill & Autopipe Integration**:
   * Connect with Harnez distill adapters (`internal/distill/`, `~/.pi/agent/extensions/harnez-distill.ts`, `~/.config/opencode/plugins/harnez-distill.ts`, and `agy` shims from Issue 195).
   * Automatically compress file content returned to the agent context using `internal/readcard` retro-pixel rendering when vision is supported.

3. **Subagent Handoff Wrapper**:
   * Ensure subagent launcher tools automatically provide helper wrappers or explicit instructions that override native file reading for large assets.

---

## 3. Acceptance Criteria

- [ ] Survey hook surfaces across `claude`, `agy`, `codex`, `pi`, and `opencode` for tool-use interception on file reads.
- [ ] Prototype a pre-tool hook / wrapper that detects multi-slice reads (>100 lines or repeated reads on same file) and triggers `harnez read`.
- [ ] Connect output distillation to generate visual PNG cards or token-bounded text.
- [ ] Add canary probe and verification test suite in `scripts/` or `internal/distill/`.
- [ ] Document transparent read interception in `docs/HookRewritePattern.md` and `docs/MultimodalContextDelivery.md`.
