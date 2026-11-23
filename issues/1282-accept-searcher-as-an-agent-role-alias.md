# 1282 — Accept searcher as an agent role alias

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: [taia #383](https://codeberg.org/harnez/taia/issues/383), #692, #893

---

## Problem
Lean-sprint uses the term searcher, but `harnez agent` reportedly does not accept `searcher` as a role alias. Existing #692 implemented general role aliases; #893 describes searcher as an advisor task.

## Goal
Accept `searcher` as an appropriate role alias and verify role resolution and relevant guidance agree. Check live code and recent history first. Stop and report on a user decision or denied permission; on a loop, hard blocker or no progress, clear the goal and record a failure report and resume point.
