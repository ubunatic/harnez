# 1085 — decide skill: session-scoped autonomy for the orchestrating agent

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 281

---

/goal A `/decide` skill and command exist that give the orchestrating agent session-scoped autonomy as specified below, and it reaches every supported agent the way other skills do, or stop and report when blocked on a user decision or denied permission. Circuit breaker: on a loop, a hard blocker or no progress, clear the goal and write a failure report here.

## Problem

The owner gets interrupted for routine choices and push confirmations during orchestrated work. Example from 2026-11-01: during the 1061 sprint and the sibling sync, the host stopped for merge and go decisions while the owner relayed answers through another session.

## Want

`/decide` puts the orchestrating agent in this mode until the session ends:

- **Push:** temporary permission to always push when a feature is ready or a milestone is reached.
- **Simple questions:** the agent answers them itself.
- **Second opinion:** consult a low-cost model.
- **Major decisions:** consult a medium or high model.
- **Very critical questions only:** stop the agent and demand user feedback.

## Open questions

- Define "very critical". Candidates: destructive or irreversible actions, force pushes, security or secret exposure, and actions outside the repo.
- How "push" relates to protected `main`: `docs/lang/Git.md` requires the admin's authorization in the current session, which `/decide` could grant. Decide whether that includes merging PRs and running `harnez release`; never force-push stays.
- Whether a peer session relaying an answer may count as the owner's decision. Today it does not.
- Name: `harnez decide` was once a compaction command (635). Check that nothing still uses it.
- Model names for each advice tier belong in `spec/`, not in the skill text.
