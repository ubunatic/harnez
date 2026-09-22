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

Defaults use the cheap models: `haiku` for claude, `gpt-6-luna` for codex (`luna` is an alias).
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

### Card style: `--card`

`bench run --read auto --card=<flags>` tells the agent, in its `AGENTS.md`, to run
`harnez read --auto <flags> <file>` (the `{{card}}` slot in the `auto` read mode of
`tasks.yaml`). The flags are the card style axes of `harnez read -I` (see below), so the same
task measures instruction following (does the agent still find and read the PNG?) and token
use (`avg_in`, `turns`). The variant is recorded in `read_mode`, e.g. `auto+style=compact`.

```
harnez bench run --agent claude --read auto --card=--style=compact --repeat 4
harnez bench run --agent claude --read auto --card="--chrome=slim --gutter=sup" --repeat 4
```

## Card style axes (`harnez read -I`)

Defaults are unchanged. Every axis is a `--flag=<mode>`; `--style=<preset>` sets several at once
and explicit axis flags override the preset.

| Flag | Modes | Effect |
|---|---|---|
| `--chrome` | `full`, `slim`, `none` | `slim`: 14px title strip and tight padding. `none`: no header, title moves into the meta box (slim strip if the box does not fit) |
| `--gutter` | `normal`, `tight`, `sup` | line numbers in the 3x5 micro font (about 16px narrower per column); `sup` top-aligns them like a superscript |
| `--frame` | `off`, `sep`, `box` | separator line above, or a box around, each section: diff files and hunks, Markdown `#`/`##` headings, Go `func`/`type`, `=== file ===` markers |
| `--meta` | `off`, `box` | red dotted info box (line range, text-token cost) in free space at the top right of the last column; only drawn when the first rows leave room |
| `--style` | `default`, `compact`, `max` | compact = slim + tight + sep + box; max = none + sup + box + box |

More ideas, not built yet: fill trailing empty space at the bottom of the last column with the
info box (next page hint, symbol index); a per-page "continues in p2 at line N" footer; dim
or collapse blank runs; a mini outline (function names) in free space; color-coded page tabs for
multi-page cards; frames for `RenderBundleCard` multi-file bundles.

First card-style sweep (claude haiku, `--read auto`, 2 runs per task, so only a smoke signal):

| Style | Pass | Notes |
|---|---|---|
| default | 3/4 | one `read-one-fact` failure |
| `--style=compact` | 3/4 | one `read-one-fact` failure, two-hop 6-8 turns |
| `--style=max` | 2/4 | both `read-one-fact` runs failed, two-hop 6 turns |

`read-one-fact` failed in every style, so the fixture read itself is unreliable at n=2; repeat with `--repeat 4` or more before ranking styles.

Font sweep (claude haiku, `--read auto`, 4 runs per task; `--card=--font=3x5` is the Tom Thumb font for all card text):

| Font | Pass | `read-one-fact` turns / avg input | `read-two-hop` turns / avg input |
|---|---|---|---|
| default 5x8 | 7/8 | 2-5 / 46k | 7-8 / 107k |
| `--font=3x5` | 8/8 | 5-6 / 87k | 6-7 / 102k |

The small font did not hurt accuracy, but the one-fact task cost about twice the input tokens and turns, so the smaller card did not turn into cheaper reads at this size. Still n=4 per cell.

Card style and font sweep on codex gpt-5.6-luna (`--read auto`, 4 runs per cell; the default cell also holds 2 earlier runs):

| Card | `read-one-fact` pass / turns / input | `read-two-hop` pass / turns / input |
|---|---|---|
| default | 6/6, 2.0, 39k | 4/6, 3.3, 76k |
| `--style=compact` | 0/4, 2.0, 39k | 4/4, 2.0, 39k |
| `--style=max` | 0/4, 2.0, 39k | 3/4, 2.0, 39k |
| `--font=3x5` | 3/4, 2.3, 41k | 0/4, 10.5, 285k |

Luna does read the PNG in the compact and max styles (2 turns, the service names come out right) but answered `12` instead of `17` for the retry limit in all 8 `read-one-fact` runs, so the slim or tight cards cost it digit accuracy. The 3x5 font made two-hop reads much more expensive (10 turns, 285k input) and failed all four. Claude haiku did not show these failures, so card style effects are agent-specific; keep the default card until a style beats it on both agents.

## Status: micro (3x5 / Tom Thumb) path shelved

Decision (2026-09-20): stop pursuing the micro font path for now. It is not the default and is not
being tuned. Evidence: `--font=3x5` doubled `read-one-fact` cost on claude haiku and failed all
`read-two-hop` runs on codex luna (10 turns, 285k input); compact/max styles (which use the micro
gutter) made luna misread `17` as `12`. The flags (`--font=3x5`, `--gutter=tight|sup`, `--style`)
stay available as opt-in experiments; the Tom Thumb digits also remain in the 5x8 superscripts.
Reopen only with a new hypothesis, for example digit-safe spacing, and re-bench on both agents.
