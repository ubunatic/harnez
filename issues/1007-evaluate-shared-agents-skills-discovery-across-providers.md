# 1007 — Evaluate shared .agents skills discovery across providers

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Skill distribution
**Related**: 981, 1006

---

## 1. Problem & Motivation
Pi reported a `codebase-memory` skill collision: `~/.pi/agent/skills/codebase-memory/SKILL.md` won over `~/.agents/skills/codebase-memory/SKILL.md`. Installing generic skills once in a shared location may avoid duplicated per-agent copies and collisions where providers support it.

## 2. Technical Specification / Findings
Determine which supported providers actually discover `~/.agents/skills` and project `.agents/skills`, their versions, precedence, collision behavior, configuration overrides and platform-specific paths. Verify using authoritative documentation and bounded live canaries; the observed Pi collision alone does not establish support by other providers. Assess skill compatibility and provider-specific variants as well as ownership and safe migration of existing customized copies.

## 3. Implementation & Verification Plan
/goal Produce an evidence-backed provider compatibility matrix and recommend how Harnez should install generic skills once where supported while retaining provider-specific paths where needed, or stop and report when blocked on a user decision or denied permission. On a loop, hard blocker, or no progress, clear the goal and record the failure and resume point here.

Evaluate collision handling, existing installs, preservation of user-managed content, status/revert behavior and migration; coordinate the ownership model with issue 1006. Propose a tested rollout rather than removing or overwriting skills before compatibility and ownership are established.
