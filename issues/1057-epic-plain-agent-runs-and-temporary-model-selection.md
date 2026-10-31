# 1057 — Epic: plain agent runs and temporary model selection

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Epic
**Related**: [942](942-epic-model-catalog-access-discovery-and-user-model-policy.md), [1027](1027-agent-context-modes-plain-no-skills-role-skills-for-pi-and-small-window-models.md), [1056](1056-set-time-limited-default-models-for-agent-effort-tiers.md)

---

## Goal

/goal Deliver minimal-context agent runs and time-limited cheap/lean/smart model defaults with optional genuinely tool-free hello probes; verify each code task with tests, install and a runtime canary. Close, skip with a reason, or park each task, or stop and report when blocked on a user decision or denied permission. On a loop, hard architectural or OS blocker, or no progress, clear the goal and write a failure report here: what was tried, why it failed and where to resume.

## Scope and dependencies

Phase 1 delivers context modes (1027), including establishing whether a no-tools, no-skills, no-injected-context invocation is possible. Phase 2 delivers temporary tier defaults and hello probes (1056), after phase 1's relevant plain-mode gate passes. Both are code tasks: each exits with its tests, install and runtime canary, a closed or explicitly parked ticket, and a clean tree. Do not assume 1027's task-specific-tool plain mode already meets 1056's stricter hello requirement.

Epic 942 retains ownership of model catalog, user policy, discovery and its bare-probe tasks 946 and 955. Coordinate shared model resolution and probe implementation with that epic; do not duplicate its tasks or silently change its decisions. No automatic permanent model-default changes, billing integration, or unrelated local-model tuning are in scope.

Open decisions: exact time syntax and tier precedence; provider support and semantics for truly bare calls; how to share work with 942. Recheck live code and recent history before refining. Web prior-art and code fanout are pending; select build versus reuse during refinement.

## Tasks

| Ticket | What | Group | Searcher → architect → developer | Reviewer | Status |
|---|---|---|---|---|---|
| 1027 | Code: agent context modes and verify minimal invocation | 1 | cheap → as needed → lean | smart | Closed (c7830529) |
| 1056 | Code: expiring tier defaults and optional tool-free hello | 2, after 1 | cheap → as needed → lean | smart | Open; refine with 942 |
| 946 / 955 | Shared bare-probe dependencies, owned by 942 | external dependency | coordinate with 942 | per 942 | Open; no duplicate ownership |
| — | Sprint study and `/evergreen` | final | host | host | Pending |
| — | `/wrap` | final | host | — | Pending |
