# 1174 — issues close: accept the implementing commit when it does not reference the ticket

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: 615, 702; study docs/studies/2026-11-06-jules-backlog-cleanup.md

---

## Problem

Backlog cleanup on 2026-11-06 verified 615 as done: `ca8bada1 feat(issues): refresh index on new ticket`
implements it, but its message does not name 615. `harnez issues close 615` refuses
(`internal/issues/status_action.go:57`): "has no commit referencing it with changes outside issues/".
The only way past is `--no-code`, which states that no code was needed, which is false here.
Old tickets done under another ticket's commit cannot be closed honestly.

## Want

- `harnez issues close <n> --implemented-by <sha> "<reason>"`: checks that `<sha>` exists and touches
  files outside `issues/`, then closes and records the sha in the Status line.

## Done when

- A test closes a ticket via an unreferencing commit with the flag, and refuses a sha that only touches
  `issues/`.
