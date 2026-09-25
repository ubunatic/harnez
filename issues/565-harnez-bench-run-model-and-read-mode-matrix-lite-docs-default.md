# 565 — harnez bench run: model and read-mode matrix, lite docs default

**Status**: Open
**Priority**: P2
**Severity**: Medium
**Category**: Bench / Tooling
**Related**: [[562-harnez-bench-agy-main-vs-helper-model-split-read-benchmark]], [[566-harnez-bench-optional-isolated-agent-config-per-run]]

## Goal

The user runs small comparisons by hand, e.g. one PNG card read vs one text read on a few models:

```
harnez bench run --task read-one-fact --read text,card --model agy:flash37:low,claude:haiku:low
```

This runs every combination (2 read modes × 2 models = 4 runs) and ends with one comparison
table for exactly these runs.

## Hard limits for the developer

- No benchmark sweeps. Live model calls only as a smoke check: at most one run per low model
  (`claude:haiku:low`, `agy:flash37:low`, `codex:luna:low`), `--task hello` or one read task.
- One `make test-q1` per turn.

## M1 — matrix and defaults

- `--model` takes a comma-separated list of `provider:model:tier` specs, the same names as
  `harnez agent models` (reuse that resolver; do not keep a second alias table). The provider
  selects the agent CLI; the tier maps to the CLI's effort/reasoning flag where it has one.
  Remove `--agent`, or keep it only as a deprecated alias if removing breaks tests beyond bench.
- `--read` takes a comma-separated list of read modes.
- Runs the full model × read × task × repeat matrix; each run keeps its own fresh temp workspace
  and (for agy) its own meter session.
- After the matrix, print one table for the runs of this invocation: model, read mode, task,
  pass, main input tokens, turns, helper tokens (agy), duration.
- `--docs` default becomes `lite`.
- Remove the unused `aggregateAgyUsage` (leftover from 562).
- Update `docs/Bench.md` (usage and example).
- Unit tests with fake runners: matrix expansion and order, spec parsing errors, lite default,
  table output. Then the smoke check within the hard limits; report its table.
- Commit `feat(bench): ... (issue 565 M1)`.
