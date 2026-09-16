# 364 — Research Quota-1 approach for LLM loops and scaffolding plan for harnez init

**Status**: Closed — resolved with quota-1 implementation and scaffolding
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [docs/practices/AgenticLoop.md](../docs/practices/AgenticLoop.md), [docs/CLIDesign.md](../docs/CLIDesign.md), [docs/HookRewritePattern.md](../docs/HookRewritePattern.md)

---

## 1. Problem & Motivation

Quota-1 is an emerging pattern for LLM agent loops designed to allow the model to only run **ONE test per step** and enforce this boundary via hooks, shims, or wrappers.

During autonomous coding loops and TDD workflows, models often struggle with test execution discipline — such as running multiple tests in a single step, re-running test suites without code edits, or spamming redundant checks.

This ticket is an open research investigation to:
1. Research and formalize the Quota-1 approach for LLM loops.
2. Formulate a design and plan for how `harnez init` can provide and scaffold Quota-1 guardrails into client repositories.

---

## 2. Research Scope & Open Questions

This investigation should research and answer the following questions from first principles without pre-committing to an assumed implementation:

### 2.1 Quota-1 Concept & Semantics
- How is Quota-1 defined across different agent architectures and literature?
- What constitutes a "step" or "turn" boundary across different harness models (Claude Code, Google Antigravity, Codex)?
- What constitutes a "test run" vs. non-test commands (compilation/build, linting, syntax checking, type-checking)?
- When and how should the quota reset (on file modification, on next user turn, on explicit state transition)?

### 2.2 Enforcement Mechanisms
- What are the viable interception layers for enforcing a single-test quota (PreToolUse lifecycle hooks, shell shims, Makefile wrappers, proxy wrappers)?
- How do different enforcement points behave under each supported agent harness?
- What should happen when the quota is exceeded (blocking with feedback, silent skip, deferral, warning)?

### 2.3 Scaffolding & Ergonomics via `harnez init`
- How should `harnez init` inject or configure these guardrails in client repositories?
- What artifacts should be generated (e.g., repository rules in `AGENTS.md`, local hook definitions, Makefile recipes)?
- Should this be an opt-in feature (e.g., `harnez init --guardrails` / `init --quota-1`) or part of default scaffolding?
- What escape hatches or bypass mechanisms are necessary for human developers, full CI runs, and Phase 3 review agents?

---

## 3. Acceptance Criteria

- [x] Complete research study documenting the Quota-1 approach, semantics, and prior art (`docs/studies/Quota1Approach.md`).
- [x] Evaluate candidate enforcement architectures across supported agent environments.
- [x] Produce a concrete design and plan for how `harnez init` will scaffold Quota-1 guardrails into repositories.
- [x] Implement and ship Quota-1 state tracking (`internal/quota1`), execution interceptor (`harnez exec --quota-1`), and scaffolding (`harnez init --quota-1`).

