# Bench

`harnez bench` is a developer harness for measuring how documentation delivery affects agent
behavior. It runs controlled fact-finding tasks under different read conditions, scores answers
with mechanical checks, and records provider usage alongside pass/fail results.

## Setup and commands

`harnez bench --setup` creates `~/.harnez/bench/bench.sqlite` and an `enabled` marker, and reports
which agent CLIs are available. Set `HARNEZ_BENCH_DIR` to use another directory. The benchmark
database is separate from the telemetry store.

- `harnez bench tasks` lists the tasks in `internal/bench/tasks.yaml`.
- `harnez bench run [--model provider:model:tier,...] [--read mode,...] [--docs full|lite] [--cards] [--task a,b] [--repeat N]`
  runs the selected model × read-mode × task × repeat matrix. Model specs use the same names as
  `harnez agent models`; `--agent` remains as a deprecated provider selector for default-model runs.
  `--docs` defaults to `lite`. One table summarizes only runs from this invocation.
- `harnez bench results [--recent N]` summarizes pass rate, average tokens, and cost by condition.

Runs invoke real providers and spend provider tokens. Use small task sets and repeat counts when
exploring a change; benchmark results are useful for comparison only when the conditions and
sample sizes are understood.

## Conditions

Each run uses a fresh temporary workspace. Its `AGENTS.md` and `CLAUDE.md` point to the task's
documentation (`base_docs` plus task-specific `docs`). The `full` and `lite` documentation modes
select the corresponding document variants where available. Card delivery sends the documentation
as PNG cards through `harnez read -I`; text delivery uses Markdown.

Read tasks compare how an agent retrieves facts from a generated fixture, with few turns. For
example, `harnez bench run --task read-one-fact --read text,card --model agy:flash37:low,claude:haiku:low`
runs four combinations and prints one comparison table:

```
model                    read     task                     pass        input   turns        helper  duration
agy:flash37:low          text     read-one-fact            PASS         1200       2             40     3.20s
agy:flash37:low          card     read-one-fact            PASS          900       2             40     3.10s
claude:haiku:low         text     read-one-fact            PASS         1100       2              0     2.80s
claude:haiku:low         card     read-one-fact            PASS          850       2              0     2.90s
```

Each matrix cell and repeat gets a fresh temporary workspace. Agy runs also receive a unique
meter session. Tier flags are passed where the provider supports effort/reasoning tiers.

Read modes:

- `native` lets the agent use its own file tools and line ranges.
- `text` instructs it to use `harnez read -n -L a:b`.
- `auto` instructs it to use `harnez read --auto`, which can return PNG cards for large content.
- `card` instructs it to use `harnez read -I`, which forces PNG output without falling back to text.

Read tasks can also vary fixture shape. `--yaml` presents the same data as one YAML file, while
`--multi[=N]` splits it into files by first letter (five groups by default). These options combine
and are recorded with the read mode so results from different shapes remain distinct.
`--card=<flags>` passes card style options to `harnez read --auto`; style axes and presets are
described in `harnez read -I --help`.

## Tasks and scoring

A task has an `id`, a prompt, optional documentation paths, and mechanical checks. The `pattern`
must match, `forbid_pattern` must not match, and every expression in `require_all` must match the
response. These are case-insensitive RE2 regular expressions; `require_all` entries can use
alternation to accept equivalent wording. Read tasks may define `read_prompts` to supply a distinct
prompt for each read mode. Fixture paths outside the generated `RUNBOOK.md` are copied from the
repository into the temporary workspace at the same relative path. Task specs are validated when
loaded, including checks that referenced document variants exist.

The task list includes documentation fact-finding and read tasks. `harnez bench tasks` displays the
available tasks. Read tasks report turns (tool calls plus the final answer) alongside provider
usage, which helps distinguish a correct answer that required more interaction from a concise one.
`read-lang-summary` exercises six language documents across native, text, and card reading prompts;
its all-keywords check requires a table, every document name, and a representative fact from each.

## Providers and recorded results

Provider invocations record the task, provider, model, documentation mode, card setting, read mode,
pass/fail, input, output and total tokens, cost, duration, and response. Claude usage includes cache
reads and creation in input; Codex usage sums completed-turn input and output. Agy runs use the
per-process agymeter, tag each run with a session ID, and read input, total tokens, and turns from
that session's meter records. Meter data is the source of truth for Agy token and turn counts.
Invocation failures, including Agy runs without meter usage, are stored with an error and excluded
from pass-rate calculations. Agy's default model is Gemini 3.7 Flash at low effort; `--model`
selects another model.

Results are grouped by condition. Compare pass rate with token use, cost, and turns, and account
for sample size and task-level failures before drawing conclusions. The harness measures these
tasks and conditions; it does not establish general model rankings. Historical measurements and
their interpretations are recorded in the [benchmark study](studies/2026-09-25-bench-read-conditions.md).
