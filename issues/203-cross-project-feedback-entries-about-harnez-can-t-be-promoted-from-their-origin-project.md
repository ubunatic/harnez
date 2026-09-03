# 203 — Cross-project feedback entries about harnez can't be promoted from their origin project

**Status**: Closed — resolved in 70bc0fd
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: [[202]], harnez feedback entry `1607041e`

---

## 1. Problem & Motivation

`harnez feedback issue` logs to a per-project JSONL file under `~/.harnez/feedback/`,
keyed by the project the calling agent happened to be in. `harnez feedback promote`
only looks up entries in the log scoped to its `-d` target, so an entry logged from
project A cannot be promoted directly into project B's `issues/`, even when its
content is actually about project B (e.g. a harnez bug observed while working in an
unrelated repo).

Concretely: while working in `smarthome`, an agent hit a harnez bug (see [[202]]) and
correctly used `harnez feedback issue` to record it — but the entry landed in
smarthome's feedback log. Turning it into a proper harnez ticket required manually
reading the raw JSONL, hand-copying the description into a fresh `issues/NNN-*.md`
file in the harnez repo, and reserving/writing the ticket by hand instead of running
`harnez feedback promote <id>`.

## 2. Technical Specification / Findings

- `harnez feedback promote <id> -d <dir>` errors with "no entry with id" when `<id>`
  exists only in another project's log.
- There is no `--from-project` / cross-log lookup option, and no flag on
  `harnez feedback issue` to target a different project's log at log time (e.g. when
  an agent already knows the feedback is about harnez itself rather than the current
  repo).
- This is a real ergonomics gap, not routine friction: the copy-over required manual
  filename derivation, hitting the exact class of bug described in [[202]] in the
  process (reserving vs. deriving a slug).

## 3. Implementation & Verification Plan

- Add a way to promote an entry by id regardless of which project's log holds it
  (e.g. `harnez feedback promote <id>` searches all `~/.harnez/feedback/*.jsonl` files
  when not found in the `-d` target's own log), or
- Add a `--project <dir>` flag to `harnez feedback issue` so an agent can log directly
  against the project the observation is actually about, rather than the project it
  happens to be running in.
- Verify by reproducing: log an entry from project A describing project B, then
  promote it into project B's `issues/` without manual JSONL inspection.

## 4. Resolution

Implemented remediation option 1 (cross-log fallback search), the smaller of the two
proposed fixes: `harnez feedback promote <id> -d <dir>` now falls back to
`feedback.FindByID`, which globs every `~/.harnez/feedback/*.jsonl` file, when the id
isn't present in `-d`'s own log. The fallback also tracks which log file the entry was
actually found in (via the new `feedback.AppendToPath`) so the "promoted" status update
is written back to the entry's real origin log, not a freshly created log for the
promotion target.

Added `TestFindByIDSearchesAcrossProjects`/`TestFindByIDUnknownIDReturnsError`
(`internal/feedback/feedback_test.go`) and `TestFeedbackPromoteCrossProject`
(`cmd/harnez/feedback_test.go`), the latter reproducing this ticket's exact scenario:
log via `runFeedbackIssue` scoped to one project dir, promote via `runFeedbackPromote`
scoped to a different `-d` target, and assert the ticket lands correctly in the target's
`issues/` while the origin log (not the target) is updated to `promoted`.
