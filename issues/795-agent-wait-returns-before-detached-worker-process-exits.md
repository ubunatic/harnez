# 795 — agent wait returns before detached worker process exits

**Status**: Draft
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**:

---

## 1. Problem & Motivation
TestAgentDetachedSpawnAndWait failed deterministically under -race (flaky
without): "worker process <pid> remains after wait returned". `agent wait`
should mean the detached worker is gone; it returned while the worker was
still alive.

## 2. Technical Specification / Findings
- A real bug, not a zombie artefact: `waitForAgent` uses `processExists`
  (kill 0) on `ProcessPID`, and the test process reaps the worker promptly.
- The worker records its terminal status and clears `ProcessPID` *before* it
  exits. `waitForAgent` then sees status != running, no PID and no writers and
  returns although the worker process is still running its exit path.
- Under -race the Go runtime sleeps ~1s at exit (GORACE atexit_sleep_ms), so
  the window is always hit; in normal builds it is a short race.

## 3. Implementation & Verification Plan
- New session fields `WorkerPID`/`WorkerStarttime`, set wherever a detached
  worker is launched and never cleared by the worker.
- `sessionRoots` adds the worker as a non-group root when it has a recorded
  starttime (PID reuse safe; zombies excluded by `TreeMembers`), so wait,
  resume and interactive-resume writer checks treat it as a writer until exit.
- Verify: `go test -race -count=5 -run 'TestAgentDetachedSpawnAndWait$' ./cmd/harnez`.
