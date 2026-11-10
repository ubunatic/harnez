---
kind: sprint
date: 2026-11-10
ticket: 1019
---

# Lean sprint 1019: Codex models unsupported for the account

## Flow

- Searcher s1019 (codex:luna:low, 54s): `candidateUnavailable` knew only denied/disabled/exhausted;
  `agent models` read quota state only; `shortCause` cut start errors to 180 runes; no parser for the
  study's "not supported when using Codex" text.
- Host decision: record a model-catalog access observation `unsupported` in the per-user state file;
  a later success clears it.
- M1 (8440796b): access writer with lock and atomic rename, models table/JSON show the state, full errors.
- Host review + reviewer r1019 (codex:terra:low, 33s): bare `model_not_found` match too broad, one
  observation per successful turn (unbounded file), full multi-line errors exposed raw tracebacks (a test
  guarding that had been flipped), no end-to-end test, a test read host state.
- M2 (bb4bcf6a): match needs the study phrase or `model_not_found` with the model id; writes only on status
  change; first error line in full, later raw lines dropped (guard test restored); cobra test with fake codex
  covers fail → table/JSON "unsupported" → success clears.

## Verification

- Developer `make test-q1` green; host reran touched packages green; `harnez init -d .` left the tree clean;
  live `agent models` unchanged for accessible models, real access.yaml holds 2 observations.

## Learnings

- A "show the full error" request silently flipped an earlier no-raw-output guard; reading changed assertions
  in the test diff caught it. "Full message" should mean the provider's message line, not its raw output.
