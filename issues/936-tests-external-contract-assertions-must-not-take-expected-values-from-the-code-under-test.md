# 936 — tests: external-contract assertions must not take expected values from the code under test

**Status**: Closed — Rule added to docs/practices/AgenticLoop.md (section 2), AgenticLoop.lite.md (item 2), root copy docs/AgenticLoop.md (synced from lite), and docs/commands/lean-sprint.md (section 2 handoff bullet). TestRootDocCopiesMatchSources PASS; internal/subagent and internal/claude packages pass. Commit 68618f27.
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Practice
**Related**: 919, 932, 934, story `docs/studies/2026-10-25-four-days-of-epics-server-monitor-and-skills.md`

---

/goal `docs/practices/` (TestTiers or AgenticLoop source, root copies synced) states that tests of an external
contract (scp/ssh operands, sandbox behavior, sample ages, real CLIs) take their expected value from the real tool
or real time, with a canary or container-tier check where needed, and the lean-sprint handoff checklist links it;
or stop and report. Circuit breaker: on a loop or no progress, clear the goal and write a failure report here.

## Evidence

- 934 M1: the test asserted the shell-quoted scp target; the live upgrade failed (`No such file or directory`).
- 932 M2: a one-shot test fixed `observed_at` an hour back and asserted the old label.
- 919: tests used an injected runner, so the systemd sandbox ssh failure was invisible.
