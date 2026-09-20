# 451 — Roadmap skill should default to low-cost models when unspecified

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: Roadmap skill

---

## 1. Problem & Motivation

The Roadmap skill may select a “pro” model by default when the user does not
specify a model. This increases cost and is inconsistent with the desired
default behavior for routine roadmap work.

## 2. Technical Specification / Findings

When no model is explicitly selected, the skill should use one of the following
low-cost defaults: `sonnet:low`, `sol:low`, or `flash37:low`. An explicitly
selected model must remain unchanged.

## 3. Implementation & Verification Plan

/goal: Update the Roadmap skill so unspecified model selection defaults to an
approved low-cost model (`sonnet:low`, `sol:low`, or `flash37:low`), while
preserving explicit model choices; verify the behavior with focused checks.

Inspect the model-selection and delegation instructions, update the default,
and verify that the documented and executable paths agree.
