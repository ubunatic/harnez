# 722 — Run Claude Code against the local lmcoder model in the agent container

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: [#723 agent container](723-consolidated-agent-container-with-in-container-lmcoder-smoke-test.md), [#073 credentialed agent canary](073-agent-canary-cloud-credentialed-claude-agy-codex.md)
**Depends on**: [#723 agent container](723-consolidated-agent-container-with-in-container-lmcoder-smoke-test.md)

---

## 1. Problem & Motivation
The #723 container tests agents against an in-container local model, so it needs
no credentials. Claude Code is the main agent Harnez manages, but it normally
talks to Anthropic's API, which is why #073 (mounted credentials) is deferred.
If Claude Code can use the local model, it joins #723 without credentials.

/goal Canary first: prove Claude Code completes a PING PONG against the
in-container lmcoder model, then add it to the #723 container and smoke test.
Stop and report if Claude Code cannot reach the local model without a fork,
cloud credentials, or an unsupported workaround.

## 2. Technical Specification / Findings
- Unverified: Claude Code can be pointed at another endpoint via
  `ANTHROPIC_BASE_URL`; whether lmcoder's llama-server or proxy offers an
  Anthropic-compatible Messages API that Claude Code accepts is open.
- Claude Code's large system prompt may make the tiny CPU model too slow or
  unreliable; record timings.

## 3. Implementation & Verification Plan
- Canary per `docs/Canary.md` before any container change.
- On success: install Claude Code in the container and add it to the smoke test.
