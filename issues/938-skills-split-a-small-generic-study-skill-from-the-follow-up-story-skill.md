# 938 — skills: split a small generic study skill from the follow-up story skill

**Status**: Open
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
- Open question: whether small studies keep all eight sections or a shorter required set.
