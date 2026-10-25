# 944 — quota-1: count targeted and coverage go test runs toward the one-run budget

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 896; gokay issue 044; gokay docs/studies/2026-10-25-two-agent-lean-sprint-gokay-skills.md

---

## 1. Problem & Motivation
gokay 044 (developer `claude:haiku:high`, 2026-10-25) ran `go test -run ... .` before `make test-q1` and `go test -coverprofile .` after it. The q1 run passed and nothing was changed between the runs. That makes three test runs under a one-run budget, and harnez neither counted nor blocked the two extra ones.

Ticket 896 did not cause this. Its exception covers only "after fixing a failure from that run", and here there was neither a failure nor a fix. The gap predates 896:
- `.harnez/rules/Quota.md` says "run the test suite once per step/turn", but never defines whether a targeted or coverage `go test` counts as that run.
- `detectQuota1` (`internal/exec/exec.go:142-159`) only activates on `--quota-1`, `HARNEZ_QUOTA1=1` or a matching env prefix. Plain `go test` is never counted or blocked.

Second opinion (`codex:sol:low`, read-only): the developer did not comply, because the "modify source before running again" line already forbids the coverage run. The texts also leave "test suite" ambiguous for targeted plain `go test`.

## 2. Technical Specification / Findings
Proposed Quota.md wording (from sol, host-agreed): "Every test execution, including targeted `go test` and coverage runs, counts toward the one-run allowance and must use the quota wrapper. Further runs require an intervening source modification; under lean-sprint §3, after fixing a failure from that run, the developer may rerun only touched packages and must report the full run owed to the host."

Coverage should come from the q1 run itself (e.g. a `-coverprofile` passed through `make test-q1` or `harnez exec --quota-1`), not from a separate run.

## 3. Implementation & Verification Plan
- M1: update the Quota.md source, the lean-sprint and lean-sprint-v1 §3 wording, and run `harnez init` so the installed rules show the change.
- M2 (enforcement): count plain `go test` / `gotestsum` runs by agents against the quota-1 state, for example through the Bash hook, and block a repeat without a source change. The 896 touched-package rerun stays allowed after a failed run.
- Verification: a test where a passing q1 run followed by `go test -cover .` with no edit is refused, and a test where a failing q1 run, a fix, and a touched-package rerun is allowed.
