# GH-prefixed ticket ids for cloud agents (GH<issue-number>) so remote agents file tickets without host help

Status: Open
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
