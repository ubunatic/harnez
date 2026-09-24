# 547 — Set HTO=0 for harnez agent sessions

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: None

---

## 1. Problem & Motivation

Agents launched with `harnez agent` should always run with `HTO=0`. Without this, session behavior can vary based on inherited environment state.

## 2. Goal

Ensure every session launched through `harnez agent` has `HTO=0`, regardless of the caller's environment.

## 3. Implementation & Verification Plan

Update the agent launch path and verify that launched sessions receive `HTO=0`, including when the caller has another `HTO` value set.
