# 1132 — Jules backlog batch: cloud sandbox and container tests (merge/close assessment)

**Status**: Closed — Batch applied, reports/jules/1132.md
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Chore
**Related**: 780, 781, 783, 784, 785, 787, 935, 1010

---

/goal Get a checked per-ticket verdict (keep, merge into another, close as done/obsolete) for the backlog tickets
below and apply it; or stop and report when blocked on an owner decision.

## Tickets (grouped by title)
- 780 Cloud agent compatibility: tests and bootstrap in sandboxed sessions
- 781 release tests require minisign on PATH
- 783 process-group kill tests fail in containers without a reaping init
- 784 agent foreground resume test fails in cloud sandbox: session not found
- 785 cloud session bootstrap: install harnez and test tools via SessionStart hook
- 787 container scripts duplicate detect_container_cmd
- 935 Add optional container test, CI (default GH), and first release plan to new-project skill
- 1010 Container integration test for `harnez server install` on a fresh user

Note: Jules itself is a cloud sandbox; where you can, run the named tests (`go test ./...`) and report the
real result for 781, 783 and 784.

## Plan
1. One Jules audit run over this ticket (report `reports/jules/1132.md`).
2. Check the report's evidence (jules skill Section 4), then close, merge or correct each ticket.

## Jules runs
| kind | session | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| batch audit | jules-batch-1132 | 963119514191714020 | GH #89 (closed) | done: 787 closed obsolete (`detect_container_cmd` only in `scripts/install-canary.sh`); 784 kept (test moved to `cmd/harnez/agent_wait_test.go`, no evidence it passes in cloud); 785 kept (`scripts/jules-setup.sh` is not the SessionStart hook asked for); 780, 781, 783, 935, 1010 kept |
