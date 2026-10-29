# 1020 — Add a read-only topic-scoped backlog value scan skill

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 978, 981

---

## Goal

/goal Provide a reusable, read-only `/value-scan` skill that helps a user understand and choose valuable work within one issue topic across repositories; verify its installed behavior and representative tracker workflows, or stop and report when blocked on a user decision or denied permission. On a loop, hard architectural or OS blocker, or no progress, clear the goal and record a failure report and resume point.

## Expected workflow

- Accept a user-selected topic; discover active tickets with the repository's documented tracker and separate direct matches from incidental mentions. Report inaccessible tracker or unreadable ticket bodies rather than guessing from titles.
- Glimpse each candidate's goal, acceptance and status; give two or three brief expected outcomes, distinguishing delivery from research. Bound large scans and disclose omissions. Follow relevant epic links without double-counting parent and child value.
- Rank by expected value to the current repository and its users, explaining criteria. Highlight useful, apparently small remaining tasks as provisional quick wins; check current code and recent history before calling a task verified easy.
- Offer next steps without changing priorities, tracker state or code. Keep the workflow user-driven and do not delegate unless requested or authorized by the skill invocation.

Unlike 978, this is a conversational topic scan, not generic triage or a workflow for moving issues to backlog. Do not require a universal scoring formula.

## Verification

Recheck live skill and tracker conventions before implementation. Verify installation and demonstrate the scan on a Harnez-tracked repository and a repository with a different documented tracker or representative fixture; include inaccessible tracker, broad topic and research-only cases. Confirm no tracker or repository changes by default and preserve user-managed content.

## Handoff

The managed skill and installation round-trip assertion are implemented. `make lint` and `make test-q1` passed; an isolated-home `harnez apply` installed the Pi skill with the expected workflow. The conversation's earlier Harnez skills-topic scan demonstrates the intended ranking, but did not exercise the newly installed skill. Live scans using the installed skill, including a non-Harnez tracker or fixture and unavailable-tracker/broad-topic cases, remain unverified; keep this issue open until those acceptance checks and an independent review are complete.
