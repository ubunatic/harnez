# 938 — skills: split a small generic study skill from the follow-up story skill

**Status**: Closed — M2 refinements committed as 2428d1de; make test-q1 passed with no --- FAIL entries in /tmp/harnez-938-m2-test-q1.log, and make install && harnez apply installed and verified both skills.
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: `docs/commands/story.md`, 905, story `docs/studies/2026-10-25-four-days-of-epics-server-monitor-and-skills.md`

---

/goal A new small `study` skill (`docs/commands/study.md`) holds what every study needs: output location and naming,
`kind:` front matter, the ticket extraction gate, and a short section list. The `story` skill keeps only the
overall follow-up story (bare `/story`: baseline, deliveries, LOC numbers, quality verdict, flow smoothness,
second-vendor review) and refers to `study` for the shared parts. Callers that write sprint reports, retros or
topic studies (`lean-sprint`, `epic`, `retro`, `sprint`, `new-project`, AgenticLoop/IssueTracking sources) point to
`study`, not `story`. Installed via `make install && harnez apply`; root doc copies synced. Or stop and report when
an owner decision blocks it. Circuit breaker: on a loop or no progress, clear the goal and write a failure report here.

## Motivation

Studies (sprint reports, retros, topics) are written often; overall stories rarely. Today every study loads the
full story skill, including the follow-up steps and the review rule, which costs tokens and pushes small studies
toward the long eight-section template.

## Notes

- Callers of `story` today: `docs/commands/{sprint,new-project,retro,lean-sprint,epic}.md`; check docs/practices too.
- Keep the `kind:` list in one place (`study`), not both.

## Decision (host, 2026-10-25)

Small studies get a shorter required set: context, outcome, what went wrong (how caught, fix), learnings,
follow-up tickets. The eight-section template stays in `story`. Reversible; recorded here.

## Sprint

Developer `d938` (sol:med), reviewer `r938` (opus:med, independent pre-close review). Searcher skipped: the
ticket names the files; the developer's plan-only first turn verifies callers and the write set.

M1 delivered (b0d0b790): study.md (40 lines, 275 words installed vs. story 760), story slimmed, six callers and
AgenticLoop full source point to study, config.yaml registration plus install test. Host `make test-q1` green.
IssueTracking needs no change (reviewer: it never named story).

## M2 — Pre-Work / Required Refinements (reviewer r938, opus:med)

1. `config.yaml:441` story description: overall follow-up project story (baseline, deliveries, LOC, quality and
   flow verdict, second-vendor review), not "retrospective and case study"; update `claudeskills_test.go:82`.
   Fix `story.md:1-3` heading/purpose the same way.
2. Name the extraction gate in study.md (heading or explicit label) so "the ticket extraction gate" in
   `story.md:40,83` and `study.md:35` resolves; say in story where allocated ticket numbers go.
3. Restore in study.md: the worker question ("What ticket number was allocated, and have all actionable follow-ups
   been filed and listed here?"), the literal `---` / `kind: <kind>` / `---` example with its reason (later
   stories find their predecessor), and that `.harnez/repo.yaml` is the indexed-studies switch.
4. Test: assert story's installed body contains its reference to study (e.g. `## Shared Study Rules`).
Optional, skipped: AgenticLoop.md:332 and jam.md links (never linked story).
