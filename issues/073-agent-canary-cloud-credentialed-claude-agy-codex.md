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

---

## Implementation Plan

### Skipped: still blocked on an external decision the agent cannot make

The blocker in §2 is a *user* decision about credential-mounting posture
(read-only vs. read-write mounts, and explicitly accepting that this container
is trusted-but-isolated rather than sandboxed from the user's real
Claude/AGY/Codex accounts). No amount of research or code resolves it, so no
implementation plan is written here beyond the §3 sequence already recorded.
Keep this ticket deferred until that decision lands.

### What is worth doing *before* unblocking (cheap, zero-risk)

1. **Answer §3 step 3 first, on paper.** Per [[070-cross-agent-distill-autopipe-hook-agy-codex-pi-opencode]],
   AGY and Codex have no real pre-exec rewrite hook — so a credentialed AGY/Codex
   container would verify only "the nudging instructions are present," which is
   checkable from a plain file assertion with no credentials, no container, and
   no blast radius. If that holds, this ticket collapses to **Claude Code only**,
   which is a much smaller credential surface (one CLI, one credential file) and
   makes the posture decision correspondingly easier to make.
2. **AGY CLI research** (install path / npm package name, currently
   unresearched) is read-only and can be done any time — but only if step 1
   concludes AGY is worth containerizing at all. Do step 1 first; it may make
   step 2 unnecessary.
3. **Reuse, do not rebuild.** Whenever this unblocks, the target is the existing
   `scripts/agent-canary/Containerfile` (already in-tree from 072: `node:22-slim`,
   no `ENTRYPOINT`, one `RUN npm install -g` layer per CLI) plus
   `scripts/agent-canary/run.sh` (already parameterized by env vars, already
   documents its podman invocation). Adding Claude Code is one `RUN` line, one
   launcher in `scripts/agent-canary/bin/`, and one `run.sh` subcommand — the
   image work is genuinely small. The cost of this ticket is entirely the
   security decision, not the build.

### Recommended framing for the unblocking decision (for the user)

- Mount credentials **read-only** and into a throwaway `HOME`
  (`/tmp/harnez-home`, which the image already sets) — never bake into the image.
- Scope the test protocol to ping-pong plus one distill-eligible tool call
  (§4), so a buggy hook rewrite has minimal opportunity to do anything with the
  credentials it can see.
- Accept explicitly that this is isolation-from-the-host, **not**
  isolation-from-the-account: a hook that exfiltrates or burns quota still can.

### Scope

**Blocked / not estimable.** Once unblocked and reduced to Claude-Code-only:
**small**. If AGY (net-new Containerfile, no prior art anywhere) stays in
scope: **medium**.
