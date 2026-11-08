# 1188 — Add /disable skill to suspend repo rules, triggered skills or process rules for a host session

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1183 (superseded by this ticket's `process quota-1` item once delivered), 1181, 1054

---

## Goal

/goal Add user-invoke-only `/disable <class> <item>` and `/enable [<class> [<item>]]` skills backed by a git-ignored state file, so the owner can suspend one named rule for the current host session and restore it explicitly. Verify both the agent-facing text and the runtime checks honour the state, that stale state is never applied silently, and that agents cannot invoke either skill; or stop and report when blocked on a user decision or denied permission. On a loop, hard architectural or OS blocker, or no progress, clear the goal and record what was tried, why it failed, and the resume point.

## 1. Problem & Motivation

In a host session the owner sometimes has to talk the model out of rules that fit the normal workflow but not the moment: Quota-1 test runs, auto-selected sprint skills, and the no-push-to-`main` rule when the owner wants a direct push. Each override is a long negotiation in chat. A named switch makes the owner's intent unambiguous and auditable.

## 2. Specification

### Syntax and initial items

| Command | Suspends | Source of the rule |
|---|---|---|
| `/disable process quota-1` | single-run limit and rerun block | `.harnez/rules/Quota.md`, `harnez exec --quota-1` (`internal/quota1`) |
| `/disable triggered-skill <name>` | auto-selection of `<name>` (e.g. `lean-sprint`, `sprint`, `factory-sprint`); explicit `/<name>` still works | skill descriptions |
| `/disable repo-rules push-main` | "do not push the default branch" | repo instructions, Repo Setup |

`/enable <class> <item>` clears one item; `/enable` alone clears all. The list of items lives in a spec file (per `docs/Spec.md`), not in Go defaults or skill text.

### Never disableable

Force-push ban, forge branch protection, rules for destructive or irreversible actions, secret handling, and spending money. `/disable` refuses these by name.

### Authority

Both skills are user-invoke only (`disable-model-invocation`). An agent may tell the owner they exist; it must never run them or write the state file itself. A peer session's relayed request is not owner authority.

### State

One file per item: `.harnez/state/disable/<class>-<item>.yaml`, git-ignored by the existing `.harnez/*` rule.

Fields: `class`, `item`, `set_at`, `session_id`, `reason` (optional, owner text).

- Same `session_id`: applied.
- Different `session_id`: stale. Not applied; the rule is enforced. The agent shows one warning naming the item and its `set_at`, and offers `/enable` (clear) or `/disable` (re-apply for this session).

### Hints and visibility

- One shared rule `.harnez/rules/Disable.md`, linked from `Index.md`, describes both skills and the user-only rule. Each affected rule or skill gets a one-line pointer to it; no copied text.
- At session start, a hook prints active and stale items in one line. No output when the directory is empty.
- The state file is the audit record; `/enable` deletes it.

### Runtime agreement

Rule text and enforcement must agree: `harnez exec --quota-1` must honour an applied `process quota-1` item, or the agent is told it may skip Quota-1 while the runtime still blocks it.

### Unverified

- Session id source per agent (Claude, Codex, AGY, Pi): check what each exposes before relying on it. If an agent exposes none, the state for it is treated as stale.

## 3. Implementation & Verification Plan

- M1: spec file with classes, items and the never-list; state read/write in `internal/<feature>` with tests for applied, stale, refused and cleared.
- M2: `/disable` and `/enable` skills, `.harnez/rules/Disable.md`, pointers in Quota.md and the sprint skills, session-start hook line.
- M3: `harnez exec --quota-1` honours an applied item; tests for normal enforcement, suspended, stale and restored.
- Close 1183 as superseded once M3 lands.
