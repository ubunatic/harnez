---
kind: sprint
date: 2026-11-26
ticket: 1298
branch: sprint/1298
pr: 226
models:
  searcher: skipped # findings checked against live code by the host the same session
  developer: claude:haiku:high
  reviewer: claude:opus # reviewer default
---

# Two-Agent Lean Sprint: #1298 — Integrate fix/init-quota-off onto main

## Context

Local branch `fix/init-quota-off` (14 commits) and a large unstaged WIP had drifted from `main`. `main` had gained 43 commits in the meantime, including the removal of runner mode and `internal/runstate` (859d787b) and an advisory Quota-1 without a sandbox (565fd744). The host classified each branch commit by trial cherry-picks in a scratch worktree, then filed #1298.

The owner decided to keep the spool feature, discard the WIP (stashed as a backup) and use `haiku:high` as developer.

## Milestones

| Milestone | Commits | Result |
|---|---|---|
| M1 spool transplant | 71fb1c32..0e365eca | 5 picks, store-error wording, spool dir tests |
| M2 quota-off init remainder | 2ab51dae..908fce76 | `managedTextFor` port, optional `rules` block, quota-on and re-init tests |
| M3 spool docs | 20adf012 | `docs/SessionIsolation.md` section |
| M4 q1 fixes | b459c9e7..3332cc87 | test-only fixes, filter-matrix guard |
| M5 review fixes | 88ef5fee..4141e7f8 | no-follow, FIFO-safe, bounded reads for spool-backed stores; wait tests |

## Verification

- Dev q1 after M3: 3 failures. Two were the sprint's own, fixed in M4. The third, `TestRunLocalSelfInstallSymlinksGoBinToLocalBin`, comes from comparing a fixture's mtime with the test binary's, and was filed as #1299.
- Host full q1 after M4 and again after M5 (post `make install`): 80 packages ok, and the only failure is #1299.
- `harnez init -d .` left a clean tree. PR #226 merged, and `harnez pr wait` exited 0.

## Observations

- **Trial cherry-picks paid off.** Picking the keeper commits onto `origin/main` in a scratch worktree before filing gave exact conflicts and a build or vet signal. That let the host skip the Searcher.
- **The plan turn corrected the ticket.** The haiku developer found that main has no `rules.quota` default: an absent `rules` block fails validation. It also found that the "spool assertions to move" were really a store-error assertion. Both were wrong premises in the host-written ticket.
- **A host diagnosis can be wrong.** M4 pre-work 2b called the spool rows in `--dead` a filter bug. The developer and reviewer showed that `--dead` means "include". Pre-work from a single log line needs the flag's meaning checked first.
- **The reviewer earned its cost.** Opus found a real check-then-use race in `agent wait` on spool-backed stores, and a FIFO hang in `agent list`, which the host's diff review had passed. This matches the 2026-11-06 lesson that one-shot reviews catch gaps the host misses.
- **Package-level isolation is not per-test isolation.** cmd/harnez `TestMain` isolates HOME once, so every registry write lands in one shared spool. Opt-in `isolateSpool` covers the listing tests. A per-test default would need a setup hook Go does not have.

## Follow-ups

- #1299 selfinstall mtime-dependent test.
- Known spool limits (documented, not fixed): pointers are keyed by session id, and only the last path part is symlink-checked.
- The spool default `~/.harnez/spool` and `DefaultStoreDir` are Go defaults, not `spec/` values.
