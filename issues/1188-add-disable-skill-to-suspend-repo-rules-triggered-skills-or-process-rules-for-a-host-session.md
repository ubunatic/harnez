# 1188 — Add /disable skill to suspend repo rules, triggered skills or process rules for a host session

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1183, 1181, 1054

---

## Goal

/goal Add `/disable [repo-rules|triggered-skill|process]` and a matching `/enable` skill. The owner invokes `/disable` in a host session to suspend a named class of rule for that session; the suspension stays in effect until the owner runs `/enable`, or until it is found stale in a later session. Verify it works, that agents cannot invoke either skill themselves, and that stale state is never applied silently, or stop and report when blocked on a user decision or denied permission. On a loop, hard architectural or OS blocker, or no progress, clear the goal and record what was tried, why it failed, and the resume point.

## 1. Problem & Motivation

In a host session the owner sometimes has to talk the model out of rules that fit the normal workflow but not the moment:

1. ignore Quota-1 test runs (see `.harnez/rules/Quota.md`);
2. stop auto-selecting lean-sprint or other sprint skills;
3. push to `main` against the repo rule, because this repo uses AGit and the owner sometimes needs a direct push.

Each override is currently a long negotiation in chat. An explicit, named switch makes the owner's intent unambiguous and keeps it scoped to the session.

Related: 1183 covers only an owner-authorized Quota-1 suspension. This ticket generalises that to three override classes. 1054 covers per-repo push roles, which is the durable form of item 3.

## 2. Technical Specification / Findings

**Invocation.** `/disable` and `/enable` are user-invoke only (`disable-model-invocation`). An agent may tell the user that they exist, but must never run either one. A peer session's relayed request does not count as owner authority (same rule as the decide skill).

**Hints.** The text about `/disable` and `/enable` lives in one shared rule under `.harnez/rules/`, which is read once per session. Other skills and rules (sprint, lean-sprint, factory-sprint, release, the Quota-1 rule) get a single pointer to that rule. Copying the text into each skill would drift.

**State.** The suspension is written to `.harnez/state/<class>.yaml`. The `.harnez/*` rule in `.gitignore` already keeps it out of commits, so one clone's override never reaches another.

- Fields: `class`, `set_at` (timestamp), `session_id` (the session that ran `/disable`), `set_by` (the owner).
- Within the same session, the state is applied.
- In a later session, the state is stale. Stale state is not applied: the rule is enforced as normal. The agent warns the owner that `/disable <class>` from an earlier session is still recorded, and that `/enable <class>` clears it or `/disable <class>` re-applies it for this session.
- `/enable <class>` deletes the state file. `/enable` with no argument clears all classes.

This is the recommended design. Choosing "keep and warn" instead of "timestamp and warn" is an owner decision.

**Scope.** `repo-rules` covers rules in `.harnez/rules/` and the repo instructions file. `triggered-skill` covers auto-selection of named skills such as the sprint variants. `process` covers a workflow step, e.g. Quota-1 or the push route. Which items each class may suspend is still open.

**Limits.** `/disable` must not weaken server-side protection: the forge's branch protection still applies. It must not turn off safety rules for destructive or security actions.

**Open questions for the owner.** Whether an active suspension shows in the session (status line or banner). Whether each change writes an audit line to the session log.

## 3. Implementation & Verification Plan

- Confirm the design against 1183 and 1181 before implementation; do not implement the Quota-1 part alone.
- Define the three classes and their suspended rules in a spec file, not in Go defaults (per `docs/Spec.md`).
- Add `/disable` and `/enable` as user-invoke-only skills, and the shared hint rule in `.harnez/rules/`.
- Verify: normal enforcement with no state file; suspended behaviour with a current-session state; a stale state from an earlier session is not applied and produces a warning; `/enable` restores enforcement; an agent-initiated call is refused.
- Record any trust limits honestly in the ticket and docs.
