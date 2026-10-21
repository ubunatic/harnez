# Two-Agent Lean Sprint: ticket hygiene (issue 843)

Date: 2026-10-22. Host: Claude Opus 5.5. Searcher `s843` (luna:low), developer `d843` (luna:high).

## Searcher

Located status parsing, filters, verbs, index and `-a status:open` references in 34 s. One claim was
wrong: "747 of 822 tickets have no parseable status". The developer's plan turn recounted with the real
parser (840/840 parseable). Lesson: counts from a searcher's ad-hoc regex need a recheck with the real code.

## Milestones

| Milestone | Commit | Outcome |
|---|---|---|
| M1 Backlog status + `backlog`/`promote` | 11b03174 | Lifecycle vocabulary in `spec/issues.yaml`; Done→Closed normalized |
| M2 duplicate gate | 029d9a6b | `find issues similar`, `issues new` blocks ≥0.55 unless `--force` |
| M3 active-only index | cbd7ec0e | README 248 KB → 67 KB, BACKLOG/CLOSED files, cap warning |
| M4 grooming | 31dfaf20 | 251 tickets → Backlog, 697 closed as dup of 698 (host-applied, user-picked list) |

## Verification (live tracker)

- `issues/README.md`: 248 KB → 5.9 KB. `find issues is:open`: 41 KB → 1.9 KB. Active: 273 → 20 after 843 closes.
- Host checked filters and verbs on a scratch tracker, and similarity scores on real text (714: 0.571,
  845: 0.733, unrelated: 0.40).
- Gap: no real "duplicate of" pairs exist, so duplicate recall rests on 697/698 and host probes.

## Efficiency

Developer time: plan 1m45s, M1 10m, M2 11m, M3 15m. One plan-only turn, no rework rounds.
Follow-up: generated-file merge conflicts (more generated files now) are tracked in 847.
