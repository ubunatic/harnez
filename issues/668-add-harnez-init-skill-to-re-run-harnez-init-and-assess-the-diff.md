# 668 — Add /harnez-init skill to re-run harnez init and assess the diff

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 667 (found by doing this workflow by hand in voxi)

---

## 1. Motivation

Re-syncing a consumer repo is a recurring manual routine: run `harnez init`, read the
diff, fix small local fallout, commit, and file real upstream bugs in harnez. On
2026-10-01 in voxi this routine found 667 (lite IssueTracking doc points to itself).
A skill makes the routine one command.

## 2. Proposal

A bundled `/harnez-init` skill that:

1. **Guard:** checks the repo was already initialized (e.g. `.harnez/rules/` exists).
   If not, it stops without running anything and asks the user to choose the init
   setup first: lite vs full docs, with or without quota1, and which docs to add.
   First-time init is a user decision, never the agent's.
2. Runs `harnez init`.
3. Assesses the diff (`git diff`): explains each change in one line, flags regressions
   (broken links, self-references, contradicting rules, lost local edits).
4. Fixes small local fallout that survives the next init; never hand-edits generated
   files that init would overwrite.
5. Files bigger problems as issues in the harnez repo, with the consumer repo and
   diff excerpt as evidence.
6. Commits the init changes in the consumer repo (no push) and reports.

## 3. Open questions

- Most reliable "was initialized" marker (`.harnez/rules/`, `harnez:begin` markers in
  AGENTS.md, or a recorded init config).
- Whether init should record the chosen variant/options so re-runs need no flags.

/goal Ship the `/harnez-init` skill with the guard and assess steps, verified by
running it in one initialized and one uninitialized repo; stop and report if blocked
on a user decision or denied permission.
