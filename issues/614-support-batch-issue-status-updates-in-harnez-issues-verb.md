# 614 — Support batch issue status updates in harnez issues verb

**Status**: Closed — batch status updates implemented
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**:

---

## 1. Problem & Motivation

Closing or transitioning multiple tickets at the end of a sprint/milestone currently requires executing individual commands for each ticket:
```bash
harnez issues close -d . 001 "reason" && harnez issues close -d . 002 "reason"
```
Supporting multiple ticket arguments in a single invocation reduces shell round-trips and simplifies orchestrator handoffs.

## 2. Technical Specification / Findings

- Update CLI parser for `harnez issues <verb>`:
  ```bash
  harnez issues <verb> -d <repo> <n...> [reason]
  # Example:
  harnez issues close -d . 001 002 003 004 005 "Completed snake v0"
  ```
- Atomically update each ticket's metadata header, resync `issues/README.md`, and commit in a single or individual structured commits.

## 3. Implementation & Verification Plan

### Goal
Allow updating multiple issue statuses in a single `harnez issues <verb>` invocation.

### Acceptance Criteria
- [ ] `harnez issues <verb>` accepts one or more ticket numbers.
- [ ] Updates all specified ticket files and `issues/README.md`.
- [ ] Unit tests covering single and multi-ticket status changes.
