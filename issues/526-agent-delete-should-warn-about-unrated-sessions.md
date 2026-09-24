# 526 — agent delete should warn about unrated sessions

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: UX
**Related**: `harnez agent rate`, `harnez agent delete`

## Goal

`harnez agent delete` must not silently discard an unrated session. Session ratings feed
model comparison, and they are lost for good once the session is gone.

## Problem

`harnez agent rate --name <s>` only works on a live session. In a loom lean sprint
(2026-09-24) the host deleted each developer session right after its milestone review, and
a later `rate` failed with `session "<s>" not found` for all six sessions (luna, flash37,
terra). Nothing warned before deleting.

## Done when

- `agent delete` on a session whose latest turn is unrated prints a warning naming the
  session and the exact `harnez agent rate --name <s> <1-5> "<reason>"` command, and refuses
  without `--force` (or an equivalent flag); the choice is recorded here.
- A test covers rated (deletes quietly), unrated (warns and refuses) and forced (deletes and warns).
- Optional: `agent list` shows a RATED column.
