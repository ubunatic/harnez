# 1104 — Point agents to harnez pr wait wherever they open a PR

**Status**: Closed — pr wait note added to Git.md, lean-sprint, factory-sprint, release, decide (cd035ad4); make test green
**Priority**: P2 (Medium)
**Severity**: Enhancement
**Category**: Agent Guidance
**Related**: [docs/commands/sprint.md](../docs/commands/sprint.md), [docs/commands/issue.md](../docs/commands/issue.md), [study](../docs/studies/2026-11-02-two-agent-lean-sprint-pr-wait.md)

---

## Problem

`harnez pr wait <number> --contains <sha>` lets an agent learn that a PR merged
without asking the owner. Only the `sprint` and `issue` command docs mention it. Agents
that open PRs from other workflows (lean-sprint, factory-sprint, release, publish, the
commit skill, the AGit Repo Setup rule, `docs/practices/` and `docs/lang/Git.md`) either
ask "is it merged?" or report the PR as done.

## Scope

Add a short, consistent note where an agent opens a PR **and** a later step depends on the
merge (a follow-up sprint, a release, a rebase, the hygiene phase). It says to start
`harnez pr wait <n> --contains <sha>` as a background command and to treat its exit code as
the only merge signal. Skip places where nothing waits on the merge, such as filing a
ticket and moving on. Edit copyable sources, not root copies (see AGENTS.md).

/goal Every PR-opening workflow whose next step depends on the merge points to
`harnez pr wait`, with one shared wording, and `make test` is green. Stop and report if
blocked on a user decision or a denied permission; on a loop or no progress, clear the goal
and write a failure report.
