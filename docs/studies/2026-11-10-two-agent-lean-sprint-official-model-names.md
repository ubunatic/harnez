---
kind: sprint
---
# Lean sprint 1203: accept official model names (2026-11-10)

## Agents

- Searcher `s1203`, codex:luna:low: one shared resolver (`internal/subagent/driver.go:720`), no duplicate names.
- Developer `d1203`, codex:terra:low: plan turn 46s, M1 3m27s, M2 3m28s.
- Reviewer `r1203`, codex:terra:low: two P2 (role gate drops diagnostics, CLI path untested).

## Milestones

- M1 (official-name resolution, ambiguity error, ranked close matches), `169e1c4a`.
- M2 (diagnostics kept through the role gate, role-path and ambiguous-alias tests), `4cbc33c7`.

## Findings

- M1's run failed only on the help golden; the developer fixed it and the host ran the owed full run (206 ok).
- Resolver tests passed while the CLI hid their diagnostics: the host's live run with a typo caught it,
  the reviewer confirmed it independently. Resolver-level tests alone did not cover the user path.
- `harnez agent start` has no dry run, so live checks used names that fail before any model start.
