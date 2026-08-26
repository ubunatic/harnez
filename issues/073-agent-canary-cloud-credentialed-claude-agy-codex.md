# 073 — Agent Canary Container: Claude/AGY/Codex With Mounted Credentials (Deferred)

**Status**: Open — deferred, blocked on credential-mounting security posture
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Agentic Ergonomics & UI Standards
**Related**: [[071-agent-canary-container-for-hook-testing]], [[072-agent-canary-local-llm-pi-opencode]], [[069-pretooluse-hook-optional-autopipe-bash-through-distill]]

---

## 1. Scope

Add **Claude Code**, **AGY** (Antigravity), and **Codex** to the same single
`harnez/scripts/agent-canary/Containerfile` from [[072-agent-canary-local-llm-pi-opencode]] (not
separate files — see [[071-agent-canary-container-for-hook-testing]] §3) — tested against their
*real cloud plans* with the user's actual credentials mounted in at `docker run` time, not a local
LLM backend. Purpose: verify issue 069's Claude Code `PreToolUse` hook (and whatever equivalent, if
any, ends up built for AGY/Codex per issue 070's finding that neither has a real pre-exec rewrite
mechanism — likely nothing to test here beyond nudging-doc presence) against the real product.

## 2. Why Deferred

- **Credential-mounting security posture is unresolved.** A buggy hook rewrite executing under
  real Claude/AGY/Codex credentials has real (if likely small) blast radius. Needs a decision on
  read-only vs. read-write mounts, and an explicit acknowledgment that this container is
  trusted-but-isolated, not sandboxed from the user's account, before any implementation.
- **No AGY (Antigravity/Gemini) Containerfile exists anywhere yet** — not in `lmcoder`, not here.
  Net-new work, including researching AGY's CLI install path/package name (unresearched as of this
  writing).
- **Per issue 070's finding, AGY and Codex have no real pre-exec hook to test.** RTK's own docs
  fall back to instruction-file nudging for both. This ticket may end up testing only Claude Code's
  hook for real, with AGY/Codex containers (if built at all) verifying nothing more than "the
  nudging doc/instructions are present" — worth confirming this is still worth building before
  investing in it.

## 3. Implementation & Verification Plan (once unblocked)

1. Resolve the credential-mounting security posture first — a standalone decision, not something
   to default into while building the container.
2. Extend the same `harnez/scripts/agent-canary/Containerfile` with one more `RUN npm install -g`
   line each for `@anthropic-ai/claude-code` and `@openai/codex` (point at the real Anthropic/OpenAI
   APIs instead of a local `llama-server` — the opposite backend choice from `lmcoder`'s
   single-agent Containerfiles, not a reuse of them as-is), plus AGY's CLI once researched (net
   new — no prior art anywhere, not in `lmcoder`, not here). Credentials are mounted at
   `docker run` time, not baked into the image.
3. Confirm whether AGY/Codex are worth adding at all given §2's third point, before investing
   further — may reduce this ticket to "Claude Code only."
4. Test protocol: same minimal ping-pong + one distill-eligible tool call as issue 072, not full
   agentic tasks.
5. Verify with a live run against the real installed-in-container CLIs, plus `harnez status`.
