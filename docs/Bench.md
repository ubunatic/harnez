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

## Read tasks (`--read native|text|auto`)

Read tasks measure how an agent reads a large doc, with few turns. The agent sees
only a bench-generated fixture (`docs/RUNBOOK.md`, 732 lines, invented values) and
the instruction from `read_modes` in `tasks.yaml`, which is the tunable text:

- `native`: the agent's own file tools with line ranges
- `text`: `harnez read -n -L a:b`
- `auto`: `harnez read --auto`, which returns PNG cards for big content

`harnez bench run --agent claude --read auto --repeat 2` runs the two fixture
tasks (`read-one-fact`, `read-two-hop`). Results show `turns` (tool calls plus the
final answer) next to tokens. First results, 4 runs per cell:

| Agent | Read | Pass | Avg turns | Avg input tokens |
|---|---|---|---|---|
| claude haiku | native | 4/4 | 2.8 | 46.5k |
| claude haiku | text | 4/4 | 3.0 | 51.3k |
| claude haiku | auto | 4/4 | 4.0 | 57.8k |
| codex luna | native | 4/4 | 3.0 | 38.0k |
| codex luna | text | 4/4 | 3.0 | 37.3k |
| codex luna | auto | 3/4 | 3.0 | 62.6k |

At this size native reads win on tokens; auto costs more, and one two-hop run took
6 turns and failed. Tune the `auto` instruction and re-run before drawing conclusions.

### Fixture shape: `--yaml` and `--multi`

Both flags need `--read` and change only how the fixture is delivered, not how it is read:

- `--yaml` delivers the whole runbook as one `RUNBOOK.yaml` (same data, `retry_limit:` style keys).
- `--multi[=N]` splits it into N files by first letter, 26/N letters per file
  (`RUNBOOK-a-f.md`, `RUNBOOK-g-l.md`, ...; empty groups are skipped). A bare `--multi`
  means 5; use `--multi=3` for other values. The flags combine.

The variant is recorded in `read_mode` (for example `auto+yaml+multi5`), so `bench results`
keeps each shape separate. The fixture is sorted alphabetically since this change, so the
whole-file `text`/`auto` cells recorded before it are not strictly comparable.

Claude haiku, 4 runs per cell (all passed): `text` 2.8 turns / 47k input tokens,
`text+yaml` 2.8 / 54k, `text+multi5` 3.0 / 50k, `auto+multi5` 4.5 / 47k. The `text`
cell also includes 4 older-order runs. Splitting files did not reduce turns.
