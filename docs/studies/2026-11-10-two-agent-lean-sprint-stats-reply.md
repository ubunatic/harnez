---
kind: sprint
---
# Lean sprint 971: stats stream always prints the final reply (2026-11-10)

## Agents

- Searcher skipped: the ticket cited file:line; the developer's plan turn re-verified it against live code.
- Developer `d971`, codex:terra:low: plan 59s, M1 3m28s, M2 test-only.
- Reviewer `r971`, codex:terra:low: one P3 (full-mode dedupe test did not count occurrences).

## Milestones

- M1 (stats prints the returned reply for codex, claude and agy; full mode adds it unless already shown), `3f1178a9`.
- M2 (dedupe test asserts the reply appears once), `71cc6ae8`.

## Findings

- The ticket was filed against an older output format; the plan turn found two more loss paths
  (Claude full mode, agy stats marker) beyond the one in the ticket.
- Live check: codex luna in stats mode through `sed -n '/\[reply/,$p'` showed the reply.
- The developer removed a "no raw CONFIRM: tag" assertion; the reviewer and host accepted it as the
  intended verbatim-reply behaviour.
