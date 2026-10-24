# 840 — lean-sprint: host decides reversible, low-risk choices itself and reports them

**Status**: Closed — Host guidance now records reversible choices in ticket decisions; verified by full suite.
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Workflow
**Related**: lean-sprint skill (Role Contract, §2 Plan First, §4 Review), 545 (lean-sprint guardrails)

---

/goal Make the lean-sprint skill tell the Host Orchestrator to decide simple, low-risk or reversible
questions itself and report each decision (one line, with where it is recorded), instead of asking
the user; keep asking only for irreversible, outward-facing or security-critical choices; or stop and
report when blocked on a user decision or denied permission.

## 1. Problem & Motivation

In a loom sprint (2026-10-21, tickets 343/349/350 and pushes) the host stopped the queue repeatedly
to ask the user: which hotkey (Ctrl+G), whether Up should also enter a menu, which of two .ansi
design variants, whether to push. All were cheap to change later (one line in `spec/*.yaml`, a
binding, a design file). The user answered "stop asking, make decisions and report them only".
Each question cost a round trip and left tickets parked.

## 2. Rule to add (user, 2026-10-21)

The host decides itself, and records the decision in the ticket (`## Decision`) and in the reply:
- Choices that are later changed in spec YAMLs: key bindings, colors, glyphs, labels, names, layout
  and placement, defaults, new spec keys.
- Design variant picks, examples, docs wording, test scope.
- Non-critical security choices (e.g. a safe default that can be tightened later).
- Routine git actions the user has already authorized in the session (e.g. pushing after each ticket).

The host still asks for: irreversible or destructive actions, outward-facing actions not yet
authorized (publishing, external issue trackers, releases), breaking API changes, and
security-relevant choices with real exposure.

Reports name the decision and the reason in one line ("343: Ctrl+G, free in spec; changeable in
spec/defaults.yaml"), never a question.

## 3. Implementation & Verification Plan

- Add the rule to the lean-sprint skill's Host invariants (and the ticket-refinement / design-round
  steps that currently say "ask the user"), plus docs/AgenticLoop.md if it lists ask points.
- Check sibling skills (`next`, `reverse-sprint`) for the same "ask before continuing" pattern.
- Verify: `harnez read` the updated skill; a dry sprint description shows no question for a
  spec-only choice.
