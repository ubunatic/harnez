# 867 — issues close: refuse when no commit outside issues/ names the ticket

**Status**: Closed — close refuses without a code commit (TestIssuesCloseRequiresCodeCommit, TestIssuesCloseBatchRefusesWithoutCodeCommit); --no-code records the override in the close commit (TestIssuesCloseNoCodeStatesItInCommit); non-close verbs unaffected (TestIssuesNonCloseVerbsIgnoreCodeGate); make test-q1 green
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: 232 (`harnez issues` verbs)

---

/goal `harnez issues close` refuses with a clear message when no commit with changes outside `issues/`
references the ticket, unless `--no-code` (or similar) states that the ticket needed no code; tests cover
both paths; or stop and report when blocked on a user decision.

## Finding (loom, 2026-10-22)

Closing 366, 323 and 367 printed "ticket N has no commit referencing it with changes outside issues/"
three times; the warning was ignored each time because the close still succeeded. Loom's developer
prompt now asks for "(issue N)" in commit subjects; a refusal would catch other repos and agents.
