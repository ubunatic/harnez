# 525 — Replace recent skill rules with harnez runtime feedback

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agent Instructions
**Related**: [[523-lean-sprint-host-runs-live-harnez-agent-checks-leaf-workers-cannot]], [[511-make-test-q1-keeps-the-full-test-log-and-prints-its-path-on-failure]], `docs/commands/lean-sprint.md`, `internal/claude/init.go` (Quota-1 block)

## Problem

Every lesson becomes one more rule line in skills and AGENTS.md; the growing text
distracts agents. The user's direction: prefer feedback baked into harnez at the moment
an agent does something wrong over more rules.

Recent rule additions that harnez could enforce instead:
- 523: "host runs live `harnez agent` checks" — leaf-role refusal can say so.
- Quota-1 "Report Untested Edits" bullet and its lean-sprint twin — `harnez exec
  --quota-1` can record the tree state; a later point (commit hook or `harnez agent`
  turn end) warns that code changed after the single run.

## /goal

Both behaviours come from harnez messages at the point of error, and the matching
rule lines are removed from `lean-sprint.md` and the Quota-1 block. Net rule text shrinks.
Future lessons default to a harnez message, a rule line only when no hook point exists.
