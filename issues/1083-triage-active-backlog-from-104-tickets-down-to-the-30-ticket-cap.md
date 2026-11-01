# 1083 — Triage active backlog from 104 tickets down to the 30-ticket cap

**Status**: Backlog — for later: P3, the active cap triage is not urgent
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: 843 (ticket hygiene, Backlog status), 882 (per-repo active cap 30), 978 (ranked grooming tool idea)

---

## Goal

/goal Bring the active tickets (Open, In Progress, Blocked, Draft) to 30 or fewer, by deferring to Backlog or closing each other ticket with a recorded reason. Done when `harnez find -d . issues -a status:active` lists 30 or fewer and commits no longer print the cap warning, or stop and report when blocked on a user decision or denied permission. On a loop, a hard blocker, or no progress, run `/goal clear` and write a failure report here with the resume point.

## 1. Problem & Motivation
104 tickets are active against the cap of 30 in `.harnez/repo.yaml` (`issues.active_cap`). Every commit prints `warning: 104 active tickets exceeds configured cap 30`, so the warning is noise until the list is triaged.

## 2. Technical Specification / Findings
- List candidates with `harnez find -d . issues -a status:active`. Re-check each one against live code and recent commits before deciding, because tickets may be stale.
- Defer with `harnez issues backlog <n> [reason]`; close with `harnez issues close <n> <reason>` only when the work is resolved in code (`--no-code` when no code was needed).
- Keep priorities as they are. Surface any priority change that looks wrong instead of making it silently.
- Ticket 978 is a tool for ranked grooming, not this one-time triage. Ticket 843 already ran a grooming pass at cap 20; it is related, not a duplicate.

## 3. Implementation & Verification Plan
1. Dump the active list and group it by priority.
2. Re-verify P3 and P2 candidates first, then the rest.
3. Defer or close in batches, one commit per batch, with the reason in each commit message.
4. Verify: the active list has 30 or fewer entries, and a commit no longer prints the cap warning.
