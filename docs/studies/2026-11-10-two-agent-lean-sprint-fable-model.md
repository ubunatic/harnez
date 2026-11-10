---
kind: sprint
---
# Lean sprint 1204: add Claude Fable 5.2 (2026-11-10)

## Agents

- Searcher `s1204`, codex:luna:low: spec template, driver `--model` pass-through, goldens; 54s.
- Web searcher `w1204`, claude:haiku:med (owner request): official price $8/$40 per MTok,
  2x Opus 5.5; 1M context; 1m12s.
- Developer `d1204`, codex:terra:low: 4 turns plus plan turn.
- Reviewer `r1204`, codex:terra:low: three findings (P2 doc row, two P3 test gaps).

## Milestones

- M1 (Fable spec entry, bench picks cheapest model), `b4e03dde`.
- M2 (tie-rule test, 80x24 picker scroll test, Haiku doc row), `036b4f95`.

## Findings

- Adding a model silently changed `harnez bench`'s Claude default from Haiku to Fable, because
  bench took the first sorted model. The developer first updated the tests to expect Fable; the host
  review rejected that. Owner chose rule B: cheapest model per provider by spec cost. Side effect:
  Codex bench moved from the global default luna:high to luna:low.
- The developer's first `skills: "?"` value failed the schema; the schema caught it in one run.
- `--role reviewer --model lean` is rejected (reviewer knows only `smart`); the sprint skill says
  "else lean". Used `terra:low` directly.
- The cost estimate went from a guess (Opus-level 20) to a sourced 40 after one web search.
