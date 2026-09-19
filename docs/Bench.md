# Bench

`harnez bench` is an optional developer harness that measures how documentation delivery
changes agent behavior. Normal users never need it. It replaces the former
`scripts/canary-agenticloop-lite`, `canary-lite-doc`, `canary-visual-doc` and
`canary-pixel-fonts` scripts with one tested feature (`internal/bench`, `cmd/harnez/bench.go`).

## Setup

`harnez bench --setup` creates `~/.harnez/bench/bench.sqlite` and an `enabled` marker, and
reports which agent CLIs are on `PATH`. Until then `bench run` and `bench results` refuse to run.
`HARNEZ_BENCH_DIR` overrides the directory. The bench DB is separate from the telemetry store.

## Commands

- `harnez bench tasks` — list specced tasks (`internal/bench/tasks.yaml`).
- `harnez bench run --agent claude|codex [--model M] [--docs full|lite] [--cards] [--task a,b] [--repeat N]`
- `harnez bench results [--recent N]` — per-condition pass rate, average tokens and cost.

Defaults use the cheap models: `haiku` for claude, `gpt-5.6-luna` for codex (`luna` is an alias).
Real runs spend agent tokens; keep task sets and `--repeat` small.

## Conditions

Each run gets a fresh temp workspace with `AGENTS.md`/`CLAUDE.md` pointing at the task's docs
(`base_docs` plus the task's `docs`). `--docs lite` swaps in `X.lite.md` where it exists;
`--cards` delivers each doc as `harnez read -I` PNG cards instead of Markdown.

## Task spec

A task has an `id`, `prompt`, optional `docs`, and a mechanical check: `pattern` must match and
`forbid_pattern` must not (RE2, case-insensitive). Specs are validated on load and a test checks
that every referenced doc exists in both variants.

## Recording

A run stores task, agent, model, docs mode, cards flag, pass/fail, token counts (claude input
includes cache reads and creation), cost, duration and the response. Invocation failures are
stored with `error` set and are excluded from pass rates.

Scope is deliberately narrow: claude and codex only (agy is not wired), regex scoring only,
one workspace per run, no cross-run statistics beyond the per-condition summary.

## First results (2026-09-19)

3 tasks (`shell-conditional`, `uncommitted-review-carryover`, `hook-unit-test-confidence`) x 4
conditions x 2 agents, one run each (`hello` smoke run excluded from the table), from
`harnez bench results`:

| agent / model | docs | cards | pass | avg input tokens |
|---|---|---|---|---|
| claude haiku | full | no | 3/3 | ~30k |
| claude haiku | full | yes | 1/3 | ~45k |
| claude haiku | lite | no | 3/3 | ~24k |
| claude haiku | lite | yes | 2/3 | ~20k |
| codex gpt-5.6-luna | full | no | 3/3 | ~28k |
| codex gpt-5.6-luna | full | yes | 2/3 | ~51k |
| codex gpt-5.6-luna | lite | no | 3/3 | ~25k |
| codex gpt-5.6-luna | lite | yes | 1/3 | ~58k |

Caveats: n=1 per cell, so this shows the harness works, not a statistically valid ranking. The
signal worth following up is that every text condition passed while cards lost points, mostly on
`shell-conditional` (the exact `if test` rule was not recovered from the card), and that cards cost
more input tokens on codex. Lite text was the cheapest passing condition on both agents.
