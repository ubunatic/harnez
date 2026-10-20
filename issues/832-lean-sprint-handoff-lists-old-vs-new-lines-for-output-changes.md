# 832 — lean-sprint: handoff lists old vs new lines for output changes

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: #699 (teardown rating, the other proposal from the same sprint)

---

## Goal

/goal Add a lean-sprint handoff checklist item: when a task changes user-visible output, the worker reports every output line it adds, changes or removes (old vs new). Ship it in the bundled skill source and verify the rendered SKILL.md; stop and report when blocked on a user decision or denied permission. Check the live skill text first; it may have changed.

## Problem

uman sprint for issue 037 (2026-10-20, uman commits `5ca99e0`, `8f38ba7`): the developer
reworked `uman pull` output and silently dropped the `integrate <name> from <remote> (<branch>)`
line. Tests only asserted the new lines, so they passed. The host caught it in diff review
after the ticket was closed, which cost an extra M2 round.

## Done when

- The lean-sprint handoff checklist has the item; the review step tells the host to compare the list against the diff.
