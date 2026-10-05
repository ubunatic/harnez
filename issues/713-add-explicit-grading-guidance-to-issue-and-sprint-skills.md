# 713 — Add explicit grading guidance to issue and sprint skills

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: [Issue tracking](../docs/IssueTracking.md), [Agentic loop](../docs/AgenticLoop.md), [#467 plan-first sprint guidance](467-recommend-a-read-only-plan-first-initial-prompt-in-sprint-skills-and-docs-without-a-template.md)

---

## 1. Problem & Motivation
Agents need to know how their work will be evaluated before they start. Add a
“How you are graded” section to the issue and sprint skills: issue-specific
criteria should be visible to the implementing agent, and sprint hosts should
tell each subagent the criteria for its delegated task.

## 2. Technical Specification / Findings
In an issue, the section should state concrete, task-specific evidence of a
successful solution, consistent with the goal and acceptance criteria. In
sprint skills, instruct the host to pass those criteria to each subagent and
make them specific to that subtask when the issue does not define them. Keep
the canonical skill sources and any generated or installed copies in sync.

## 3. Implementation & Verification Plan
/goal Update the canonical issue and sprint skills so implementers see how an
issue solution will be graded and sprint hosts tell subagents how their work
will be graded; verify the guidance is present in the relevant sprint variants
and issue workflow, or stop and report when blocked on user input or denied
permission.
