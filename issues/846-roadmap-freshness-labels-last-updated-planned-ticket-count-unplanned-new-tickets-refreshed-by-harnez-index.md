# 846 — Roadmap freshness labels: last updated, planned ticket count, unplanned new tickets, refreshed by harnez index

**Status**: Backlog — grooming 2026-10-23: active cap 20; no work planned this week
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 234 (/roadmap skill), 843 (ticket hygiene), docs/Roadmap.md

---

/goal `docs/Roadmap.md` always shows a prominent freshness label that `harnez index` keeps
current, verified on the live repo, or stop and report when blocked on a user decision.

## Problem

On 2026-10-22 the roadmap's "Now" bucket was stale (7 of 10 items closed, last planning pass
2026-09-27) and nothing in the file said so; the host only noticed while choosing the active set
for 843. Readers need to see at a glance how current the plan is.

## Wanted

A prominent label at the top of `docs/Roadmap.md` (front matter or a generated block):

- **Last updated**: date of the last planning pass.
- **Planned with N known tickets**: tickets the roadmap references.
- **M new tickets never considered**: open tickets filed after the pass, or open tickets the
  roadmap does not mention.
- Possibly: K referenced tickets now closed (stale entries).

`harnez index` recomputes the numbers on every run; the `/roadmap` skill sets "last updated"
and the known-ticket baseline when it does a planning pass.

## Open questions

- Store the baseline (pass date, known ticket set) in front matter or derive it from ticket
  references in the body? Body references are free but fuzzy (ranges, "250–256").
- Generated block markers (like the existing `harnez:begin/end` blocks) vs. YAML front matter.
- Should other harnez-managed projects with a `docs/Roadmap.md` get it too (opt-in)?
