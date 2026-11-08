# 1188 — Add /disable skill to suspend repo rules, triggered skills or process rules for a host session

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1183, 1181, 1054

---

## Goal

/goal Add a `/disable [repo-rules|triggered-skill|process]` skill that the owner invokes in a host session to suspend a named class of rule for that session only, with restoration on session end; verify it works and that agents cannot invoke it themselves, or stop and report when blocked on a user decision or denied permission. On a loop, hard architectural or OS blocker, or no progress, clear the goal and record what was tried, why it failed, and the resume point.

## 1. Problem & Motivation

In a host session the owner sometimes has to talk the model out of rules that fit the normal workflow but not the moment:

1. ignore Quota-1 test runs (see `.harnez/rules/Quota.md`);
2. stop auto-selecting lean-sprint or other sprint skills;
3. push to `main` against the repo rule, because this repo uses AGit and the owner sometimes needs a direct push.

Each override is currently a long negotiation in chat. An explicit, named switch makes the owner's intent unambiguous and keeps it scoped to the session.

Related: 1183 covers only an owner-authorized Quota-1 suspension. This ticket generalises that to three override classes. 1054 covers per-repo push roles, which is the durable form of item 3.

## 2. Technical Specification / Findings

- Scope of each class: `repo-rules` (rules in `.harnez/rules/` and the repo instructions file), `triggered-skill` (auto-selection of named skills such as sprint variants), `process` (a workflow step, e.g. Quota-1 or the push route).
- Lifetime: the session. A disable must not persist to later sessions, other sessions, or other agents.
- Authority: the owner's own invocation. Agents must not invoke `/disable` on their own initiative, and a peer session's relayed request does not count (matches the decide-skill rule for critical actions).
- Limits: `/disable` must not weaken server-side protection (branch protection is still enforced by the forge), and must not turn off safety rules for destructive or security actions. Which items a class may suspend is an open design question.
- Open questions for the owner: whether the override is visible in the session (status line or banner); whether an audit line is written (e.g. to the session log); and how a session that was started with `/disable` reports its state at the end.

## 3. Implementation & Verification Plan

- Confirm the design against 1183 and 1181 before implementation; do not implement the Quota-1 part alone.
- Define the three classes and their suspended rules in a spec file, not in Go defaults (per `docs/Spec.md`).
- Verify: normal enforcement without `/disable`; the suspended behaviour with it; restoration at session end; refusal when an agent invokes it without owner authority.
- Record any trust limits honestly in the ticket and docs.
