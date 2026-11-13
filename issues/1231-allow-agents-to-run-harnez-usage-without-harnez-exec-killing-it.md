# 1231 — Allow agents to run harnez usage without harnez exec killing it

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 541, 533, 1230

---

## 1. Problem & Motivation
In agent sessions, `harnez usage` (also `go test ./internal/usage/` and `make install`'s follow-up steps) exits 125 with `harnez exec: process group remained stopped after SIGCONT; sent SIGKILL`, printing no usage. Agents cannot read quota, so they cannot verify usage changes (seen while implementing 1230) or pick models by remaining quota.

## 2. Technical Specification / Findings
- Likely cause: the stopped-group monitor from 541/533 treats a descendant that stops (probably the `claude -p /usage` child or a test that raises SIGSTOP/SIGTTIN) as a stuck run and kills the group.
- Reproduces on the unmodified baseline, so it is not a regression of 1230.
- Unconfirmed: which descendant stops, and whether this is the same on all hosts. Find out first.
- Fix at the cause (e.g. keep the usage child from stopping, or give `harnez usage` a sanctioned path in `harnez exec`). Do not weaken the stuck-process guard for other commands.

## 3. Implementation & Verification Plan
- Reproduce with `harnez exec -- harnez usage` and identify the stopping process.
- Fix it; add a regression test.
- Verify in an agent session: `harnez usage` and `harnez usage --minimal -w` print quota; `go test ./internal/usage/` completes.

**/goal** Make `harnez usage` run to completion when called by agents, with a regression test, or stop and report when blocked on a user decision or denied permission.
