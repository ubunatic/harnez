# 617 — Create /respect skill for spec-driven implementation

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: `spec/` YAML files

---

## 1. Problem & Motivation

Agents often implement constants, parameters, and other values directly in code even when they belong in the repository's YAML spec. This is easy to correct later, but it makes the spec-driven approach unreliable. The spec is the YAML in `spec/`, embedded at compilation and included in the binary; it is not runtime configuration or prose documentation.

## 2. Technical Specification / Findings

Create a skill named `/respect`. It should instruct an agent to explore the requested repository, module, or package; find hardcoded constants, parameters, and other values that belong in the YAML spec; move those values into the spec; and update implementation to consume the embedded spec.

## 3. Implementation & Verification Plan

**Goal**: Add and document `/respect` so agents can bring implementation in line with the embedded YAML spec. Done when the skill clearly directs this workflow and its scope. Stop and report if the repository's spec structure or ownership of a value is unclear and needs user input.
