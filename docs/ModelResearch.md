# Model Research

How to refresh the model guidance in `spec/agent.yaml` (`cost`, `eff`, `skills`, `roles`,
`use`, `use_med`) when something changes. The result is an updated spec, a new dated
snapshot in [Models.md](Models.md) and a checked `harnez agent models` table. Evidence
from earlier runs: [Models.md](Models.md), [ModelAdvisoryEval.md](ModelAdvisoryEval.md);
role practice: [practices/ModelRoles.md](practices/ModelRoles.md).

## Triggers

- A new model or id appears in a provider's model list (Codex: `slug` in
  `~/.codex/models_cache.json`).
- A price or subscription quota change.
- Repeated role failures of one model in sprint retros.

## Scope rule

Re-run only the new or changed models. Re-run all models only when an anchor changes:
`codex:luna` (COST 1) or `codex:astra` (COST ~100× luna, level 8).

## Steps, cheapest first

Stop after the step that answers the trigger. Run from the harnez repo unless a step says
otherwise.

### 1. Id check (no tokens)

Compare every `name:` in `spec/agent.yaml` against the provider's own list, never against
docs or web research.

```bash
jq -r '.. | .slug? // empty' ~/.codex/models_cache.json | sort -u
grep -n 'name:' spec/agent.yaml
```

For claude and agy, check the ids the CLI accepts (`claude --model <id>`, the agy model
picker). Fix wrong ids first; a wrong id makes every later step measure the wrong model
(`codex:astra` once pointed at a nonexistent `gpt-5.6-astra`).

### 2. Column-wise web research (~30–50k new tokens and 25–55 s per researcher)

One researcher per **column**, covering all models on one scale; not one per model,
because per-model agents produce ratings that don't line up (conflicting sol/terra
prices). Columns: COST, EFF. Do not web-research Go, TUI or SQLite: there is no evidence
for any model, so a rerun only re-labels guesses (use step 3).

Run from a scratch dir outside the repo, on `luna:med`:

```bash
mkdir -p /tmp/model-research
harnez agent start --name mr-cost --role advisor --model codex:luna:med \
  -d /tmp/model-research --stream stats "$(cat prompt-cost.txt)"
```

Prompt template (swap the column block for EFF):

```text
Web research only. Do not inspect any repository or local files.
Models: <list provider ids from spec/agent.yaml, e.g. gpt-6-luna, gpt-6-sol, ...>.
Column: COST. Rate every model on one scale: level 1-9, each step ~2x the previous,
anchors gpt-6-luna = 1 and gpt-6-astra = 8 (~100x luna). Give list price per 1M
input/output tokens and, separately, any subscription/quota evidence; list price is
not subscription cost.
[EFF: tokens needed to reach a coding goal: + few, ~ average, - many, ? no data.]
Output: one table row per model: model | value | evidence | source URL.
Mark ? when you found no source. At most 30 lines.
```

Then overlay user-stated cost ratios (they outrank list price) and keep both anchors.

### 3. Repo-local skill canaries (Go, TUI, SQLite)

Small tasks with known answers, run on each candidate at its configured tier, scored by
accuracy and tokens per goal (read tokens with `--stream stats`; compare within one vendor
only). SQLite canary: a known schema, known answers, and a fan-out join trap that
double-counts. There is no canary script yet (open question in issue 514); write the task
into a scratch ticket and grade by hand. Cost: one developer turn per model, roughly
100–300k input tokens, mostly cached.

### 4. Advisory eval (role changes only)

When a trigger may change `roles`, run "Evaluating models" from
[practices/ModelRoles.md](practices/ModelRoles.md): one identical advisory prompt, graded
against the repo with a fact-check grid. Cost per model: ~50–300k input tokens, 1–2 min
([ModelAdvisoryEval.md](ModelAdvisoryEval.md) "Cost and speed").

### 5. Update spec and snapshot

- Edit `cost`, `eff`, `skills`, `roles`, `use`, `use_med` in `spec/agent.yaml`; keep the
  header comment and `models_legend` in sync if a scale changes.
- Add a dated `## YYYY-MM-DD <topic> snapshot` section to [Models.md](Models.md): method,
  table, findings, research cost. Never rewrite older snapshots.

### 6. Check and commit

```bash
make install
script -qc 'harnez agent models' /dev/null   # real pseudo-terminal: widths and colors
make test-q1
```

Commit spec and docs together, e.g. `docs(models): <date> COST refresh for <model>`.

## Cleanup

Read quota numbers first (per-turn readings live in the session rollouts and are deleted
with the session), then delete every research session and the scratch dir:

```bash
harnez agent delete --name mr-cost
harnez agent delete --name mr-eff
rm -rf /tmp/model-research
```
