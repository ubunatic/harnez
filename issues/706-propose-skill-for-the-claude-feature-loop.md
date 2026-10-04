# 706 — Propose skill for the Claude feature loop

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**:

---

## 1. Problem & Motivation
The feature loop in the supplied reference image is useful but currently lacks a reusable skill: Claude interviews to clarify a feature, builds it with low effort, the user reviews it, and Claude verifies it with high effort. The cycle repeats, with effort adjustable mid-conversation.

## 2. Technical Specification / Findings
Treat this as a proposed skill. Explore how to encode the four steps and effort changes in a concise, reusable workflow; record uncertainties rather than assuming tool-specific commands or implementation details.

## 3. Implementation & Verification Plan
/goal Create and verify a proposed skill that guides the Claude interview → low-effort build → user review → high-effort verification loop, or stop and report when blocked on a user decision or denied permission.

Verify the skill has clear stage guidance, supports iteration and mid-conversation effort changes, and follows the applicable skill conventions.
