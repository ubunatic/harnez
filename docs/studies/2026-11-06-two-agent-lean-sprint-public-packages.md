---
kind: sprint
date: 2026-11-06
tickets: [1177, 1173, 1138, 1139]
---
# Lean sprint 1177, 1173, 1138, 1139: public Go packages for taia, then small fixes

## Context

- Owner request: developer `claude:haiku:med`, `terra:low` as one-shot reviewer/consult; host picks
  the extra tickets (decide mode). Branch `sprint/1177-1173-1138-1139` (git mode team).
- 1177: export the model catalog and the `usage --json` types as stable module-root packages for taia
  (`ubunatic.com/harnez/modelcatalog`, `ubunatic.com/harnez/quota`). 1173, 1138, 1139: small, tested
  fixes filed this session.
- Searcher skipped for 1177: this session's import check (`internal/modelcatalog` 2 harnez deps,
  `internal/usage` 18) already names the write set; plan-only developer turn first.

## Outcome

- 1177 M1 (catalog): `dev-1177` plan turn running.

## What Went Wrong

None yet.

## Learnings

To be filled.

## Follow-Up Tickets

None yet.
