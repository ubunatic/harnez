# 523 — lean-sprint: host runs live harnez agent checks; leaf workers cannot

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Documentation
**Related**: `docs/commands/lean-sprint.md`, `docs/practices/AgenticLoop.md` (roles), `docs/studies/2026-09-24-sprint-519-agent-stats.md`

## Problem

In the 519 sprint two developer prompts asked for an end-to-end `harnez agent`
start + resume check. Leaf roles are refused `start/resume/delete`, so both workers
reported "blocked" and the host re-ran the check. AgenticLoop.md now says the
orchestrator runs such checks; the lean-sprint command doesn't.

## /goal

`docs/commands/lean-sprint.md` says in the dispatch and review steps that live checks
needing an agent session are run by the host after the developer's commit, and that
dispatch prompts must not ask workers to run them.
