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

- 1177 M1 (catalog), `dev-1177` claude:haiku:med: plan turn found `Load` already exported and all
  callers in `internal/subagent`. `0d8c9607` moved 42 files without content change, `88241c72` added
  the stable-API doc and an external `Load` test. One-shot review (codex:terra:low, 43s): no
  blockers; flagged a weak test and a deprecation note without a replacement.
- 1177 M2 (quota types): plan turn found three staleness methods that need the YAML spec; host chose
  option A (package functions in `internal/usage`, not on the stable API). `1e4617b2` captured a
  byte-identity golden of `RenderJSON` before the move, `c9e82502` moved the types (about 25 call
  sites), `75ac394b` added the doc and a decode round-trip test. `go list -deps ./quota`: standard
  library only. Review (28s): golden missed `error` and `weekly`; `HasUsageData` and a `json:"-"`
  field were inside the promise.
- 1177 M2b (`cc849662`): golden covers `error` and `weekly`; `HasUsageData` moved to `internal/usage`;
  doc excludes `json:"-"` fields. 1177 closed; M3 (exported collector) parked as 1178.
- 1173 (Jules config error message): fresh `dev-1173` claude:haiku:med, plan turn running.

## What Went Wrong

- **Estimates of call-site churn.** M2's plan said about 13 call sites, the move touched about 25.
  Caught by the compiler and the golden test, not by review.
- **The cheap reviewer earned its turn twice.** Both one-shot terra:low reviews found real gaps the
  host review had passed (weak external test; golden coverage and API scope).

## Learnings

To be filled.

## Follow-Up Tickets

None yet.
