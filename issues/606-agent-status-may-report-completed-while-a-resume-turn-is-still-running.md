# 606 — agent status may report completed while a resume turn is still running

**Status**: Closed — resume persists running state before the provider turn; a second resume on a live running session is refused, a dead PID is treated as stale (db244209)
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug (confirmed)
**Related**: 603 (sprint where it was seen)

---

`/goal`: Confirm or rule out that `harnez agent status` shows `completed` during an active resume turn; fix it if it does, or stop and report when it cannot be reproduced.

## 1. Problem & Motivation
During the 603 M3 sprint (2026-09-27) the host ran `harnez agent resume --name peer-603 ...` detached with shell `&`
(a mistake, now forbidden in AgenticLoop). Right after, `harnez agent status --name peer-603` printed
`Status: completed`, although the resume was running (`codex exec resume <id>` alive, it later committed dc86ac5).
A second resume then failed with codex `thread-store conflict: ... already has an active writer`.
A host that trusts `status` would start a conflicting second writer.

## 2. Technical Specification / Findings
- Confirmed on HEAD: synchronous `runResume` called the provider without first saving `running`, so status continued
  to show the prior `completed` state; a second resume could therefore start a conflicting writer.
- Fixed by persisting `running` and the current PID before the provider starts. A live PID now blocks another resume;
  a dead PID is treated as stale. Success, failure, and token-watchdog interruption persist a final state and clear
  the PID.

## 3. Implementation & Verification Plan
- Reproduced and verified with a blocking fake driver: `agent status --json` reports `running` mid-turn; a second
  resume is refused with a clear active-turn message; completion clears the PID. A reaped child PID verifies stale
  session recovery. Resume failure coverage verifies `failed` state and PID cleanup.
- `make test-q1`: expected configuration-related failures only (17 `cmd/harnez` tests and 1 `internal/claude` test)
  from the pre-existing, uncommitted `jev_compaction_enabled: true` edit in `config.yaml`.

## 4. Peer Report (loom, 2026-09-27)
- loom ran `harnez agent resume` in a foreground Bash call; `harnez exec` killed it after 1m
  ("timeout kill after 1m0s; rerun with HTO=0 to lift") mid-turn, partial edits left on disk, and
  `agent status` then showed `completed`. Second trigger for the same bug: a turn that was killed or
  interrupted must be reported as `interrupted`/`failed`, not `completed`.
- Guidance given: run `resume` as a tracked background job (Claude `run_in_background`) with `HTO=0`;
  check with `agent status` / `agent wait`. A `resume --detach` flag is not planned (581 removed detaching).
