# 1281 — Show forge issue comments to workers

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: [taia #383](https://codeberg.org/harnez/taia/issues/383), feedback 51bdb4c6, #1207

---

## Problem
`harnez issues show --forge` reportedly prints only the issue body, hiding comments containing feedback needed by workers. #1207 covers adding comments, not reading them.

## Goal
Make forge issue comments accessible to workers through `issues show`, with clear output and regression coverage. Check live code and recent history first. Stop and report on a user decision or denied permission; on a loop, hard blocker or no progress, clear the goal and record a failure report and resume point.
