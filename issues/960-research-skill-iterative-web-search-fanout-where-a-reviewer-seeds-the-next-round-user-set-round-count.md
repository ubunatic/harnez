# 960 — research skill: iterative web search fanout where a reviewer seeds the next round (user-set round count)

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 900 (epic web fanout), 941 (models websearch fanout)

---

/goal Ship a skill (e.g. `/research`) that runs an iterative web search fanout: N cheap parallel searchers →
host report → stronger reviewer that corrects the report and proposes the next round's search angles →
repeat for a user-set number of rounds → final reviewed study. Verified by one real run with 2 rounds. Stop and
report when blocked on an owner decision; on a loop or no progress, clear the goal and write a failure report.

## Why

The flow was run by hand in snapdex on 2026-10-26 (study
`snapdex/docs/studies/2026-10-26-web-fanout-detection-answers-evidence.md`): a context brief with what was
already learned, 4 `haiku:med` searchers with one angle each, a host-written report, a `sol:med` reviewer
that checks claims against the sources and suggests 4 new angles with search terms, then one more round.
Seeding the next round from the reviewer goes deeper than one flat fanout, and the brief stops searchers
from re-finding what is known. Today `epic` (900) has only a single-round prior-art fanout.

## Shape (to refine)

- Inputs: topic, the "already learned" brief, rounds (default 1, user-set), searcher and reviewer models
  (defaults cheap / mid-tier, different vendors).
- Each searcher: one angle, read-only, linked findings with date, license, cost and a fit verdict;
  "no link, no finding".
- Reviewer per round: corrections with evidence, ranking changes, exactly K next angles with terms.
- Report grows per round in `docs/studies/` (`kind: study`), noting which round found what.
- Stop early when the reviewer finds no new angle worth searching.
- Searchers must not ask the user for "go"; observed: all four haiku searchers ended with "say go".
