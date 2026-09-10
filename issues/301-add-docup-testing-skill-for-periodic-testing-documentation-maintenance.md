# 301 — Add /docup testing skill for periodic testing documentation maintenance

**Status**: Open — deferred pending review and assessment
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Documentation
**Related**: `docs/Testing.md`, `AGENTS.md`, `d248997`, `docs/commands/`

---

## 1. Problem & Motivation

Testing guidance is now available in [`docs/Testing.md`](../docs/Testing.md) and
is linked from `AGENTS.md`, but there is no focused workflow for periodically
reviewing whether that guidance still matches the repository. Over time,
testing commands, test layers, canaries, and verification expectations can
change while the documentation remains stale or difficult for an agent to
audit quickly.

Create a future Harnez skill, `/docup testing`, for this maintenance task. The
skill should give an agent a short, repeatable way to inspect the current
testing documentation, identify concrete drift or omissions, and update the
relevant documentation when the review justifies it.

Implementation is intentionally deferred until the issue has been reviewed and
the workflow, scope, and integration points have been assessed.

## 2. Technical Specification / Findings

The command structure should be organized by documentation-maintenance
category:

- `docs/commands/Docup.md` — the parent command and shared workflow;
- `DocupTesting.md` — the first category, focused on quickly reviewing and
  updating testing documentation; and
- later categories such as `DocupArch.md`, `DocupReadme.md`, and other
  narrowly scoped documentation-maintenance workflows as they are justified.

The first category should direct an agent to inspect the relevant testing
guidance, compare it with current repository behavior and conventions, surface
specific discrepancies, and make bounded documentation edits with appropriate
verification. It should account for the existing testing layers described in
`docs/Testing.md`, including package tests, integration-style tests, static
checks, smoke tests, canaries/live checks, and manual or visual verification.

The assessment should resolve how the parent command discovers category
documents, how categories are registered or linked, whether the command is
Claude-specific or shared across supported agents, and how the workflow avoids
turning a periodic documentation review into an unbounded repository audit.

## 3. Implementation & Verification Plan

- Review and assess the proposed `/docup` parent/category command structure
  before implementation.
- Define the shared workflow in `docs/commands/Docup.md` and the first
  category workflow in `DocupTesting.md` only after that assessment.
- Make the testing workflow identify its source documents, relevant code,
  commands, tests, canaries, and recent changes before proposing edits.
- Require concrete findings and bounded updates; preserve accurate existing
  guidance when no change is warranted.
- Verify command discoverability, document links, formatting, and the
  resulting testing guidance with the repository's documentation checks.
- Add later categories only as separate, reviewed extensions of the same
  structure.

No skill, command, or code implementation is included in this ticket's filing
change; this ticket records the future work for review and assessment.
