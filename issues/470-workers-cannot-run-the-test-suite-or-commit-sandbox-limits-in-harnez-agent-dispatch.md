# 470 — Workers cannot run the test suite or commit: sandbox limits in `harnez agent` dispatch

**Status**: Closed — resumed workers now run with the same sandbox bypass as start (1c3f49a); developers ran go test, make install and committed; Quota-1 stays enforced and is documented in OrchestratedAgentFlow.md
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agent dispatch / Workflow
**Related**: [291](291-add-harnez-agent-command-standardized-non-interactive-dispatch-to-external-coding-agent-clis-codex-agy.md), [342](342-mvp-harnez-agent-command-and-skill-for-cross-agent-dispatch-agy-host-to-codex-subagents.md), [468](468-compact-make-test-q1-output-to-summary-and-failing-tests-growing-harnez-distill.md), [176](176-structured-capped-subagent-completion-report-contract.md)

---

## Problem

Codex workers started via `harnez agent start` run with a read-only `.git`, read-only Quota-1
state and Go cache. Across three sprints (2026-09-21) they could not commit or run
`make test-q1`, so the host committed and ran the tests. Workers used filtered `-run` patterns
with a scratch `GOCACHE`, which missed tests they had just added (127 M2, 341), and sometimes
reported sandbox artifacts as pre-existing failures (462: two `exec_test.go` "failures" that
passed under the host).

## /goal

Decide and document how dispatch handles this, then make the workflow consistent. Options to
evaluate: (a) grant workers a writable `.git` and quota state under an explicit sandbox policy
(291's `--sandbox`), keeping `QUOTA_BYPASS` off-limits; (b) keep workers read-only and make the
handoff contract explicit: host runs `make test-q1` once per change, workers verify the whole
package (never a filtered `-run`) with a scratch `GOCACHE`, and known sandbox-only failures are
listed so they are not misreported. Record the choice in `docs/AgenticLoop.md` and the sprint
skills.

## Notes

- The host-runs-tests model worked in all three sprints; the defects were the filtered runs and
  the misreported failures, not the split itself.
- Pair with 468 so the single host run is easy to read in full.
- Re-verify against live code before starting.
