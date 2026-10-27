# 988 — Track retrieval, prompt and answer evidence separately in sprint handoffs

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: 972 (verify-first); [Neus issue 102](../../neus/issues/102-improve-exact-symbol-and-test-inclusive-caller-retrieval-in-ask.md); [Neus acceptance audit](../../neus/docs/studies/2026-10-27-issue-102-acceptance-audit.md)

---

## 1. Problem & Motivation
In the Neus #102 lean sprint, a test caller appeared in retrieval results but was
lost during prompt assembly. Later, a model gave a citation-valid answer that
mischaracterized a callback. Several developer handoffs and test runs targeted
different boundaries before the final prompt and source-supported claims were
checked together. A ranking score or valid citation alone did not establish that
the answer had the required evidence or used it correctly.

The host needed a compact, durable record for each saved case: what retrieval
found, what survived in the final prompt, what the answer claimed against source,
and which of those checks were not run. That would have made the remaining work
clear to each fresh developer and helped stop repeated fixes at the wrong boundary.

## 2. Technical Specification / Findings
This applies when a sprint changes retrieval, evidence selection or grounded
answers; it should not burden unrelated tickets. Capture artifact paths and the
source/index snapshot, backend and model where relevant. Keep factual source
adjudication distinct from citation-format checks and aggregate ranking metrics.
An unavailable capture must be recorded as unverified, not as a pass.

## 3. Implementation & Verification Plan
**Goal**: `/goal Add a lightweight evidence handoff and review check for applicable
retrieval and grounded-answer sprints, and verify it against the Neus #102 case;
or stop and report when blocked on a user decision or denied permission. On a
loop, hard architectural or OS blocker, or no progress, clear the goal and write
a failure report.`

- Inspect the current sprint and lean-sprint guidance; extend the appropriate
  shared handoff/review point rather than duplicating rules across skills.
- For each applicable saved case, record retrieval, final-prompt presence, and
  source-adjudicated answer outcome separately, with pass/fail/unverified and
  links to evidence. Carry unresolved boundaries into the next handoff.
- Replay the Neus #102 example: a retrieved caller omitted from the prompt and a
  cited callback misdescribed as a direct call must remain distinct failures;
  unrelated sprint handoffs should not require the extra record.
