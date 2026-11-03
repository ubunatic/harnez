# 1109 — Jules runs show two IDs: local UUID in agent list, numeric Jules ID elsewhere

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**:

---

## Problem
Reported 2026-11-03 by the loom session.

`harnez agent list` shows a local UUID (e.g. `5441328c-...`), while the run output and `harnez agent status --model jules --id` use the numeric Jules ID (`4295776973374618428`). It is unclear which one goes into the `## Jules runs` table and `--id`.

Mitigated: the /jules skill now says to record the numeric Jules ID.

## Goal
/goal `agent list` (and its JSON) shows the Jules ID next to the local ID for Jules sessions; stop and report on a blocker.

## Done when
- `harnez agent list` prints the provider session ID for jules rows; test covers it.
