# 978 — Groom open issues into backlog with a ranked candidate list

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: Neus local issue ranking

---

## Goal

Enable quick grooming of an oversized open issue list: propose a requested number of tickets to defer, review the ordered candidate numbers, and move chosen tickets to Backlog using the existing tracker workflow. Consider adopting this flow in `harnez issues`. Verify the live tracker and current implementation before starting. Stop and report if blocked on a user decision or denied permission; on a loop, hard blocker, or no progress, clear the goal and report the failure and resume point.

## Context and acceptance

- A single `luna:med` prompt containing 46 issue numbers, titles, statuses, and priorities produced 15 numbered deferral candidates ordered by priority. Deferring 15 of 46 leaves 31 open, not 30.
- Explore a fast, reviewable workflow that collects issue summaries, accepts a target open count or number to defer, and proposes distinct valid candidates without repeating titles. Let the user review or adjust the proposal before status changes.
- Respect priority and surface deviations; handle model failures, abstentions, invalid or duplicate numbers, and count mismatches. Do not silently close tickets. Preserve existing batch backlog, index, and commit semantics for approved choices.
- Verify selection and tracker integration with representative tickets and requested counts. Evaluate a local Neus driver when available, without assuming it can rank reliably today.
