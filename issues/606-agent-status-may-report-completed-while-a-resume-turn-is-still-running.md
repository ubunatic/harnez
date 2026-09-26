# 606 — agent status may report completed while a resume turn is still running

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug (unconfirmed)
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
- Unconfirmed: status may have been read before the resume registered its turn (race), or status may reflect only
  the last finished turn. The `&` launch may have affected it.

## 3. Implementation & Verification Plan
- Reproduce: start a long resume in a tracked background shell, query `agent status` during the turn.
- If it shows `completed`: status must report `running` while a turn's process is alive, and `resume` should refuse
  with a clear message (not the raw codex error) when a turn is active.
- Test with a fake driver holding a turn open.

## 4. Peer Report (loom, 2026-09-27)
- loom ran `harnez agent resume` in a foreground Bash call; `harnez exec` killed it after 1m
  ("timeout kill after 1m0s; rerun with HTO=0 to lift") mid-turn, partial edits left on disk, and
  `agent status` then showed `completed`. Second trigger for the same bug: a turn that was killed or
  interrupted must be reported as `interrupted`/`failed`, not `completed`.
- Guidance given: run `resume` as a tracked background job (Claude `run_in_background`) with `HTO=0`;
  check with `agent status` / `agent wait`. A `resume --detach` flag is not planned (581 removed detaching).
