# 896 — lean-sprint: one-run q1 budget needs a rerun rule after a post-run fix

**Status**: Draft
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Docs
**Related**: 887, 894; docs/commands/lean-sprint.md §3; docs/studies/2026-10-24-two-agent-lean-sprint-epic-894-vendor-mixed.md

---

/goal lean-sprint §3 says what a developer does when its single `make test-q1` run fails and it fixes the
failure: rerun once, or hand the rerun to the host and say so; done when the skill states one rule; stop and
ask if the quota budget rules (`.harnez/rules/Quota.md`) conflict.

## Problem

887 M1 (epic 894 Sprint 2): the one q1 run found a stale assertion, the developer fixed it, ran only the
focused package and committed. Harnez then warned that a test file changed after the last q1 run. The host
reran q1 (green). The skill's "one run" wording leaves this case open.
