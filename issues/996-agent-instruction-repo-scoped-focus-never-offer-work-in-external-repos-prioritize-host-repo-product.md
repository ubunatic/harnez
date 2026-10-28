# 996 — Agent instruction: repo-scoped focus — never offer work in external repos, prioritize host repo product

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Policy
**Related**: [979](issues/979-epic-agent-instructions-and-help-text.md), [972](issues/972-claude-guesses-and-pushes-to-implement-instead-of-verifying-add-a-verify-first-instruction.md), [963](issues/963-teach-agents-to-ask-where-rules-belong-before-editing-agents-md.md)

---

## 1. Problem & Motivation

Agents operating in a specific repository frequently experience "context drift": when an issue or discussion mentions an external dependency, sibling repository, or generic tool (e.g. `goself`, `harnez`), the agent volunteers or offers to implement features or fix bugs in that external repo.

This breaks core product boundaries:
1. **The host repository's applications are the agent's product**: The active repository's codebase and features must always have strict priority over any external distractions.
2. **Unauthorized Cross-Repo Scope Expansion**: An agent should *never* offer to perform implementation or coding tasks in an external repository. Only the human user can explicitly demand work in another repository.
3. **Allowed Cross-Repo Actions**: The only permissible action an agent may independently take in an external repository is passive reference lookup, or filing an issue / ticket when explicitly directed or needed to unblock a dependency. The agent must never propose switching its own dev loop to that external repo.

## 2. Technical Specification / Findings

Define and inject a generic, standard instruction across Harnez-managed agent templates (`docs/AgenticLoop.md`, `spec/instructions/`, and generated `AGENTS.md` / `CLAUDE.md` blocks):

### Rule Invariants
- **Product Primacy**: The applications, tools, and code inside the current repository are the sole product of this agent session. They always take precedence over external distractions.
- **Repository Boundary**: Never proactively offer, volunteer, or initiate implementation or coding tasks in another repository.
- **User-Mandated Only**: Cross-repository coding tasks may *only* be executed if the user explicitly orders and scopes them.
- **Read-Only / Tracker Boundary**: Outside the host repository, an agent may only read for context, inspect dependencies, or file an issue when requested or required. Never pivot the active implementation focus away from the host repo.

### Injection Points
1. Add the rule to the Harnez Managed Conventions block in `AGENTS.md` / `CLAUDE.md` (e.g. under *Process Hygiene* or *Agentic Loop Invariants*).
2. Include the rule in the core system prompt templates in `harnez agent` dispatch.
3. Update `docs/AgenticLoop.md` with an explicit invariant on **Repository Scope & Product Focus**.

## 3. Implementation & Verification Plan

1. Draft the exact instruction wording in `docs/AgenticLoop.md` and `internal/spec/conventions.go` (or Harnez managed instruction templates).
2. Verify that running `harnez init` / `harnez agent` reflects the new boundary instruction.
3. Validate agent behavior in multi-repo workspaces to ensure agents reject distractions and decline unprompted cross-repo work.

/goal Add the generic repo-scoped product focus instruction to harnez agent templates and conventions so agents never offer cross-repo work, or stop and report when blocked on a user decision or denied permission. On a loop, hard architectural blocker, or no progress, clear the goal and write a failure report explaining why it failed.
