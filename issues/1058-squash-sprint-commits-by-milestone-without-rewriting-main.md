# 1058 — Squash sprint commits by milestone without rewriting main

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Enhancement
**Related**: 981

---

## Problem & Motivation

Sprint work can leave many incremental commits. Add an end-of-sprint squash step to lean-sprint and factory-sprint so history has one commit per milestone or larger coherent concern.

## Goal and Scope

/goal Update both sprint workflows and verify their instructions preserve meaningful milestone or concern boundaries while never squashing or rewriting commits already on main. Stop and report when blocked on a user decision or denied permission; on a loop, hard architectural or OS blocker, or no progress, clear the goal and write a failure report with the resume point.

Recheck live workflow sources, repository state and recent history before implementation. Define a safe boundary against current main, respect protected branches and shared or published history, and avoid losing unrelated work. Stop and ask when safe rewriting cannot be established; do not force-push implicitly. Keep ticket references and review/verification evidence traceable after squashing.

## Verification

Check both workflow instructions and installed or generated counterparts as applicable. Verify boundary and safety guidance with representative histories, including commits already on main and multiple milestones, and document constraints.

Recovered from unpublished commit `7248728f`; its original issue number 1053 was independently allocated on main.
