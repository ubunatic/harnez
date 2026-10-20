# Detect duplicate ticket numbers after merges and offer renumbering

Status: Closed — Reject duplicate numbers before index/status proceeds and rewrite exact ticket filename references during mv, with regression tests; implemented in b3e933b5
Priority: P2
Category: Feature

## Problem
Ticket numbers are allocated from each writer's own snapshot. Merging diverged histories
produced duplicates: cati 096 and 097 each had two files after the Codeberg/GitHub merge
(renumbered by hand to 100-102). A cloud agent working from GitHub also picked harnez 805
while a local worker had just filed 805-808.

## Proposal
- `harnez index` / `harnez status` report duplicate numbers as an error.
- `harnez issues renumber -d <repo> <file> [new]` moves a ticket to the next free number and
  rewrites references (issues/, docs/, commit-message-free).
- Document the host-reserves-numbers rule for remote agents (docs/OrchestratedAgentFlow.md §6).

## Acceptance
- A tracker with two `096-*.md` files fails `harnez index` with both paths named.
- Renumber updates the file, its title line, README index and in-repo references; tested.

## Delivery

- Implementation: `b3e933b5`; documentation: `62519fe4`. The existing command is
  `harnez issues mv`, not a new `renumber` alias.
- `index` (including `--check`) and `status` reject duplicate numbers with every
  colliding path and the `harnez issues mv -d . <file>` repair hint. Index validates
  before any README write. Existing `issues lint` duplicate detection is retained.
- Moves rewrite exact filename/path references in issues/ and docs/ in the same
  commit, preserving bare numbers and longer filenames. Missing numbered headers
  report the offending path and how to add the header before retrying.
- Tests first reproduced the index/status, reference, and missing-header failures.
  Temp-repo regressions cover two/three claimants, committed references, punctuation,
  bare numbers, check-mode restoration, and rollback after README validation fails.
- Persistent lock files are intentional: unlinking a flock sidecar can split writers
  across different inodes. Existing init code and tests already ignore both
  `/issues/README.md.lock` and `/issues/.reserve.lock` in `.git/info/exclude`.
- Verification passed: `make lint`, `go test ./internal/issues/ ./cmd/harnez/`,
  `make test-short`, `make install`, and root-doc sync via `harnez init -d .`.
  Lint reports existing advisory warnings only. No push; no open delivery gaps.
