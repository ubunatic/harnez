# Model Research

How to refresh the model guidance in `spec/agent.yaml` (`cost`, `eff`, `skills`, `roles`,
`use`, `use_med`) by **web research** when something changes. No model is run on tasks
here: cheap researchers read public benchmarks, vendor docs and practitioner reports. The
result is an updated spec, a new dated snapshot in [Models.md](Models.md) and a checked
`harnez agent models` table. Earlier snapshots: [Models.md](Models.md).

## Triggers

- A new model or id appears in a provider's model list (Codex: `slug` in
  `~/.codex/models_cache.json`).
- A price or subscription change.
- New benchmark results for a skill column (Go, TUI, SQL) or a sprint retro that
  contradicts a rating.

## Scope rule

Research only the new or changed models, plus the two anchors `codex:luna` (COST 1) and
`codex:astra` (COST 100) so the new values land on the same scale. Research every model
only when an anchor itself changes.

## Steps

### 1. Id check (no tokens)

Compare every `name:` in `spec/agent.yaml` against the provider's own list, never against
docs or web research. A wrong id makes the research describe the wrong model (`codex:astra`
once pointed at a nonexistent `gpt-5.6-astra`).

```bash
jq -r '.. | .slug? // empty' ~/.codex/models_cache.json | sort -u   # verified 2026-09-23
grep -n 'name:' spec/agent.yaml
```

Claude and agy have no verified list command yet; check their ids by hand (their CLI's
model picker) and note it in the snapshot.

A new model also needs a spec entry (provider, name, tier) and a check whether its CLI
accepts an effort flag (`effort: false` otherwise, as for `agy:sonnet`/`agy:opus`).

### 2. One researcher per column (~30–50k new tokens, 25–55 s each)

Give each column to its own researcher covering all models in scope, so one agent sets one
scale. Per-model researchers produced ratings that didn't line up (conflicting sol/terra
prices on 2026-09-23). Columns and what to ask for:

| Column | Ask for | Expect |
|---|---|---|
| COST | list price per 1M input/output tokens, subscription/quota evidence, as a multiple of luna | good public data |
| EFF | tokens or cost per solved task (e.g. Artificial Analysis, Terminal-Bench cost figures) | partial |
| Go, TUI, SQL | language- or domain-specific benchmarks (e.g. BIRD, Spider 2.0 for SQL) | thin: keep `?` unless a source is model-specific |

Run all researchers in parallel from a scratch dir outside the repo, on `codex:luna:med`,
role advisor (a web-only researcher role is issue 510):

```bash
d=$(mktemp -d)
for col in cost eff go tui sql; do
  harnez agent start --name mr-$col --role advisor --model codex:luna:med -d "$d" \
    --stream stats "$(sed "s/<COLUMN>/$col/" prompt.txt)" > "$d/$col.txt" 2>&1 &
done
wait
```

Prompt template:

```text
WEB RESEARCH ONLY. Do not inspect any repository or local files; run no local commands
except web search.
Models (provider ids): <ids in scope, always including gpt-6-luna and gpt-6-astra>.
Column: <COLUMN>. Rate every model on ONE shared scale:
- cost: multiple of gpt-6-luna, anchors gpt-6-luna = 1, gpt-6-astra = 100; list price per
  1M input/output tokens; subscription/quota evidence separately (list price is not
  subscription cost).
- eff: tokens needed to reach a coding goal: + few, ~ average, - many, ? no data.
- go / tui / sql: + strong, ~ ok, - weak, ? no model-specific evidence. Ignore Python,
  JS/TS and web results.
Output: one table row per model: model | value | evidence | source URL.
Mark ? when you found no source. Flag conflicting sources. At most 30 lines.
```

### 3. Reconcile

- Researchers' tables are agent-reported: spot-check the sources behind any value that
  changes a spec rating.
- User-stated cost ratios (e.g. opus ≈ 2× sonnet) outrank list prices; keep both anchors.
- A `?` stays `?`: don't turn "no data" into `-`.

### 4. Update spec and snapshot

- Edit `cost`, `eff`, `skills` (and `use`/`roles` if the evidence changes a role) in
  `spec/agent.yaml`; keep the header comment and `models_legend` in sync if a scale changes.
- Add a dated `## YYYY-MM-DD <topic> snapshot` section to [Models.md](Models.md): scope,
  table, findings, research cost. Never rewrite older snapshots.

### 5. Check and commit

```bash
make install
script -qc 'harnez agent models' /dev/null            # real pseudo-terminal
make test-q1 > "$d/q1.log" 2>&1; grep -- '--- FAIL' "$d/q1.log"
```

Commit spec and snapshot together, e.g. `feat(agent): <date> model research refresh`.

### 6. Cleanup

```bash
for col in cost eff go tui sql; do harnez agent delete --name mr-$col -d "$d"; done
rm -rf "$d"
```

## Not part of this plan

Running models on known-answer tasks (skill canaries) or the repo-graded advisory eval
([practices/ModelRoles.md](practices/ModelRoles.md) "Evaluating models") gives stronger
evidence but costs a turn per model; use them only when web research can't settle a role.
