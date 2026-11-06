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
- 1173 (Jules config error message), fresh `dev-1173`: plan turn probed `encoding/json` live
  (`UnmarshalTypeError` vs `SyntaxError`). `68d9afaf`: a syntax error gives its byte offset, a type
  mismatch names the expected shape, the decoder text is never echoed; four tests incl. key secrecy.
- 1138 (forge 403/404 hints), fresh `dev-1138`: host decision to retry a 403'd read without the token;
  the developer improved it: on a second failure return the original 403, since a private repo answers
  404 anonymously. `dbeaa516` shared `forge.GetJSON`; host live check in a scratch clone: `issues new`
  with the stored read-only token no longer reports "open PRs not checked ... 403". Review added two
  missing tests (`8c6e875b`: 401/5xx no retry, 403-then-404 keeps scope advice).
- 1139 (forge `status:` filters), fresh `dev-1139`: `c4292192` carries `state` to the forge. Host live
  check: `find -d ../taia issues status:open` lists exactly taia's 4 open issues; `status:closed skills`
  ANDs both; `status:backlog` names the supported values.
- One combined one-shot review (codex:terra:low, 24s) for 1173, 1138, 1139: no blocker or secret leak.
- All touched packages green after the last commit; helpers rated and deleted.

## What Went Wrong

- **Estimates of call-site churn.** M2's plan said about 13 call sites, the move touched about 25.
  Caught by the compiler and the golden test, not by review.
- **The cheap reviewer earned its turn twice.** Both one-shot terra:low reviews found real gaps the
  host review had passed (weak external test; golden coverage and API scope).

## Learnings

- claude:haiku:med was a good developer for scoped work: every plan turn verified locations live,
  asked the right API questions, and stayed inside the write set.
- A one-shot terra:low reviewer (24-43s, about 25-35k new tokens) found a real gap in every review;
  host review alone would have shipped weaker tests three times.
- Fresh developers per independent ticket kept each session small, below Haiku's 100k price step.
- Byte-identity goldens captured before a move are the cheapest proof that a refactor kept output.

## Follow-Up Tickets

- 1178: export a quota collector without server, telemetry or TUI deps (1177 M3).
