# Benchmark read conditions and card experiments

This study preserves the dated measurements and decisions formerly included in
[`docs/Bench.md`](../Bench.md). The small samples are exploratory observations, not general
provider rankings.

## Initial documentation delivery results (2026-09-19)

Three tasks (`shell-conditional`, `uncommitted-review-carryover`, `hook-unit-test-confidence`),
four documentation conditions, and two agents were run once per cell. A `hello` smoke run is
excluded. Values are from `harnez bench results`.

| Agent / model | Docs | Cards | Pass | Average input tokens |
|---|---|---|---|---|
| claude haiku | full | no | 3/3 | ~30k |
| claude haiku | full | yes | 1/3 | ~45k |
| claude haiku | lite | no | 3/3 | ~24k |
| claude haiku | lite | yes | 2/3 | ~20k |
| codex gpt-5.6-luna | full | no | 3/3 | ~28k |
| codex gpt-5.6-luna | full | yes | 2/3 | ~51k |
| codex gpt-5.6-luna | lite | no | 3/3 | ~25k |
| codex gpt-5.6-luna | lite | yes | 1/3 | ~58k |

With one run per cell, this only established that the harness worked. Text conditions passed in all
cells while cards lost points, mostly on `shell-conditional` (the exact `if test` rule was not
recovered from the card); cards also cost more input tokens on codex. Lite text was the cheapest
passing condition for both agents.

## Read conditions

Initial read-task results used four runs per cell:

| Agent | Read | Pass | Average turns | Average input tokens |
|---|---|---:|---:|---:|
| claude haiku | native | 4/4 | 2.8 | 46.5k |
| claude haiku | text | 4/4 | 3.0 | 51.3k |
| claude haiku | auto | 4/4 | 4.0 | 57.8k |
| codex luna | native | 4/4 | 3.0 | 38.0k |
| codex luna | text | 4/4 | 3.0 | 37.3k |
| codex luna | auto | 3/4 | 3.0 | 62.6k |

At this fixture size, native reads used fewer tokens; auto cost more, and one two-hop run used six
turns and failed. The auto instruction needed tuning before conclusions could be drawn.

### YAML and multi-file fixture shapes

Both options require `--read` and change fixture delivery, not the read method. `--yaml` turns the
runbook into a single `RUNBOOK.yaml`; `--multi[=N]` splits it into N files by first letter, with
five groups by default. The variant is included in `read_mode` (for example `auto+yaml+multi5`).
Alphabetical fixture sorting was introduced after the earlier whole-file `text` and `auto` runs,
so those cells are not strictly comparable.

Claude haiku, four runs per cell, all passed: `text` 2.8 turns / 47k input tokens, `text+yaml`
2.8 / 54k, `text+multi5` 3.0 / 50k, and `auto+multi5` 4.5 / 47k. The `text` cell also includes
four older-order runs. Splitting files did not reduce turns.

### Card style and font

The first card-style sweep used Claude haiku, auto reads, and two runs per task:

| Style | Pass | Notes |
|---|---:|---|
| default | 3/4 | one `read-one-fact` failure |
| `--style=compact` | 3/4 | one `read-one-fact` failure; two-hop took 6–8 turns |
| `--style=max` | 2/4 | both `read-one-fact` runs failed; two-hop took 6 turns |

`read-one-fact` failed in every style, making the fixture read unreliable at n=2. More repeats
were needed before ranking styles.

Claude haiku font sweep, auto reads, four runs per task (`--card=--font=3x5` uses the Tom Thumb
font for all card text):

| Font | Pass | `read-one-fact` turns / average input | `read-two-hop` turns / average input |
|---|---:|---:|---:|
| default 5x8 | 7/8 | 2–5 / 46k | 7–8 / 107k |
| `--font=3x5` | 8/8 | 5–6 / 87k | 6–7 / 102k |

The small font did not hurt accuracy, but the one-fact task cost about twice the input tokens and
turns, so the smaller card did not make reads cheaper at this size. The sample was n=4 per cell.

Card style and font sweep on codex gpt-5.6-luna, auto reads, four runs per cell (the default cell
also includes two earlier runs):

| Card | `read-one-fact` pass / turns / input | `read-two-hop` pass / turns / input |
|---|---|---|
| default | 6/6, 2.0, 39k | 4/6, 3.3, 76k |
| `--style=compact` | 0/4, 2.0, 39k | 4/4, 2.0, 39k |
| `--style=max` | 0/4, 2.0, 39k | 3/4, 2.0, 39k |
| `--font=3x5` | 3/4, 2.3, 41k | 0/4, 10.5, 285k |

Luna read the PNG in compact and max styles (two turns, correct service names) but answered `12`
instead of `17` for the retry limit in all eight `read-one-fact` runs. The 3x5 font made two-hop
reads more expensive (10 turns, 285k input) and all four failed. Claude haiku did not show these
failures, so card style effects were agent-specific; the default card remained the recommended
choice until a style beat it on both agents.

## Micro font path decision (2026-09-20)

The micro font path (3x5 / Tom Thumb) was shelved and was not made the default or tuned further.
Evidence: `--font=3x5` doubled `read-one-fact` cost on Claude haiku and failed all `read-two-hop`
runs on codex luna (10 turns, 285k input); compact/max styles, which use the micro gutter, made
Luna misread `17` as `12`. The flags (`--font=3x5`, `--gutter=tight|sup`, `--style`) remained
available as opt-in experiments; Tom Thumb digits also remained in the 5x8 superscripts. The path
could be reopened with a new hypothesis, such as digit-safe spacing, followed by benchmarks on
both agents.
