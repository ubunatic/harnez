# 699 — Rate lean-sprint sessions before teardown

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: #526

---

## 1. Problem & Motivation
The lean-sprint skill's teardown currently deletes developer sessions without first
rating them. `harnez agent delete` now warns and refuses when a session is unrated,
which turns teardown into an avoidable extra round trip and can leave cleanup
unfinished.

## Goal

Update lean-sprint teardown to rate each completed agent session before deleting it,
and verify the documented sequence. Stop and report if the rating command or session
state prevents a valid rating.

## 2. Technical Specification / Findings
The current teardown in the installed lean-sprint skill calls
`harnez agent delete --name <session_id>` directly. Ticket #526 added the unrated
session warning and refusal, including the suggested rating command.

## 3. Implementation & Verification Plan
- Document the rating step before session deletion in lean-sprint teardown.
- Ensure teardown handles the rating result before proceeding with deletion.
- Verify the skill instructions consistently describe the rating-then-delete sequence.
