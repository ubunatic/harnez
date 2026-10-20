# GH-prefixed ticket ids for cloud agents (GH<issue-number>) so remote agents file tickets without host help

Status: Closed — Shipped permanent GH ticket allocation and tracker support with fake-gh coverage; required checks passed in 1259f057
Priority: P2
Category: Feature

## Problem
Ticket numbers come from each writer's local snapshot. A cloud agent working from GitHub picked
harnez 805 while a local worker filed 805-808 (2026-10-17); the host had to reserve numbers by
hand (809, loom 334, cati 109, voxi 194). Merges of diverged histories also produced duplicates
(cati 096/097). User decision: cloud agents must move without host help.

## Proposal
- Remote/cloud agents file tickets as `issues/GH<n>-<slug>.md`, where `<n>` is the GitHub issue
  number they create for it (globally unique per repo, so no collisions), or a GH-local counter
  when no GitHub issue is wanted.
- `harnez issues new --remote` (or auto-detect: env `CLAUDE_CODE_REMOTE`/no local tracker lock)
  allocates the GH id; `harnez index`, `find`, `issues close` and README accept both id forms.
- Optional: `harnez issues renumber GH<n>` converts to the next local number on merge, rewriting
  references; or GH ids stay permanent.
- Document in docs/IssueTracking.md and the rules distributed by `harnez init`.

## Acceptance
- A cloud agent can create, reference and close `GH42` without touching local numbering.
- Local `harnez issues new` never collides with GH ids; index sorts both; tests for both forms.
- Related: 814 (duplicate detection after merges).

## Delivery

- Implemented in `1259f057` (817): `issues new --remote "Title"` uses authenticated
  `gh issue create` against the `github` remote and links the GitHub body to the exact
  `issues/GH<n>-<slug>.md` path. `CLAUDE_CODE_REMOTE` being set selects remote mode;
  the selected mode prints to stderr. Missing gh/authentication fails without a local fallback.
- GH IDs stay permanent, support case-insensitive arguments and all normal tracker operations,
  sort numerically after local IDs, and are checked for duplicates and header mismatches.
  Local allocation ignores GH IDs; `42` stays local while `GH42`/`gh42` select the remote ID.
- Updated full/lite IssueTracking sources, synced root copy, OrchestratedAgentFlow §6,
  config.yaml distributed rule sources and generated Tools/Issues rules; refreshed CLI help golden.
- Tests first used an injected fake gh runner; covered GH42 allocation, create/edit failures,
  malformed gh output, missing gh/remote/title, environment selection (including empty), local
  mode, JSON, close/find/index/lint, mixed ordering, unchanged local allocation with GH900,
  duplicate GH42 rejection, permanent IDs and filename/header mismatch. No real GitHub issues created.
- Verification passed: `make lint`, `go test ./internal/issues/ ./cmd/harnez/`,
  `make test-short`, `make install`, and `git diff --check`. The first package run exposed the
  expected help-golden drift; the snapshot was regenerated and the package checks passed.
- Allocation spans GitHub and disk: failures after GitHub creation report the existing issue URL;
  they do not allocate a local fallback. A failed body update writes no local ticket.
