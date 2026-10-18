# Detect duplicate ticket numbers after merges and offer renumbering

Status: Open
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
