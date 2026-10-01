# 674 — test: TestAgentDetachedSpawnAndWait flaky under full-suite load (cannot establish ownership of process)

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: [[656-harnez-agent-send-opt-in-messaging-to-detached-agents-after-host-session-agent-tracking]]

---

## 1. Problem & Motivation

During the 656 M1 fix (2026-10-01, commit a7d010f4), one full `make test-q1` run failed
`TestAgentDetachedSpawnAndWait` with "cannot establish ownership of process". The test passes when
run alone, with and without the M1 change, and passed in the next full run. It's flaky under load.
A flaky test burns the quota-1 single test run.

## 2. Technical Specification / Findings

Not yet investigated. Probably a race between the detached spawn and the ownership check (process
start time or pid file not yet written) when the machine is busy.

## 3. Implementation & Verification Plan

Reproduce with `go test -run TestAgentDetachedSpawnAndWait -count=50 ./cmd/harnez` under parallel
load, find the race, fix the wait/ownership handshake, not the assertion.
