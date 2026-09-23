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

### 2. Researchers: one per model family, plus one for price and efficiency (~30–50k new tokens, 25–55 s each)

Every cell of `harnez agent models` is a hypothesis to confirm or update. Split so that no
researcher repeats another's identity work, and one researcher owns each cross-model scale
(design reviewed by `codex:astra`, 2026-09-23):

| Researcher | Rows / columns | Owns |
|---|---|---|
| `mr-gpt6` | luna, sol, astra | identity, effort controls, SKILLS, ROLES/USE claims |
| `mr-gpt56` | terra | same |
| `mr-claude` | haiku, sonnet, opus (claude and agy routes) | same |
| `mr-gemini` | flash37, flash38 | same |
| `mr-price` | all models | COST and EFF on one scale |

Per-model researchers (2026-09-23 first sweep) produced ratings that didn't line up;
per-column researchers repeat the identity work for every column. Family researchers plus
one scale owner avoid both. Skip families outside the trigger's scope.

Run in parallel from a scratch dir outside the repo, on `codex:luna:med`, role advisor
(a web-only researcher role is issue 510). Paste the current table into every prompt:

```bash
d=$(mktemp -d); harnez agent models > "$d/models.txt"
for r in gpt6 gpt56 claude gemini price; do
  harnez agent start --name mr-$r --role advisor --model codex:luna:med -d "$d" \
    --stream stats "$(cat "prompt-$r.txt" "$d/models.txt")" > "$d/$r.txt" 2>&1 &
done
wait
```

Family prompt (`prompt-<family>.txt`):

```text
WEB RESEARCH ONLY. Do not inspect any repository or local files.
The table below guides model choice for agentic coding. Audit rows: <rows>.
For each row: verify the model exists under that exact id, its version, provider route and
effort controls (what low/med change; whether EFFORT "no" means unsupported). Then audit
every SKILLS (Go, TUI, SQL; ignore Python, JS/TS, web), ROLES and USE claim: turn each
phrase into a checkable statement ("slow first token" = measured first-token latency) and
look for model-specific evidence. Never infer one route (e.g. agy vs claude) from another.
Output a ledger, one line per claim: row | claim | verdict (confirmed/contradicted/?) |
evidence with date | source URL | confidence. At most 40 lines.
```

Price and efficiency prompt (`prompt-price.txt`):

```text
WEB RESEARCH ONLY. Do not inspect any repository or local files.
For every model in the table below:
COST: official input/output/cached/reasoning token prices; subscription price, quota and
reset rules and any overage. Price one fixed workload (100k input, 20k output, 50%
cached) per model and report it as a multiple of gpt-6-luna = 1. Report astra's measured
ratio; do not force it to 100. Keep cash price and quota consumption separate; do not
invent cross-vendor quota conversions.
EFF: tokens (including retries) per SUCCESSFUL task from a named harness and task suite
(e.g. Terminal-Bench or SWE-bench cost figures). Reject leaderboard scores or speed as a
substitute; "?" if no such measurement exists.
Output one line per model: model | list price | workload ×luna | quota notes | EFF value
| evidence | source URL. At most 40 lines.
```

### 3. Reconcile

- Source ranking: official pricing and model docs for facts, reproducible independent
  evaluations for capability; marketing, anecdotes, affiliate posts and unsourced rankings
  are leads only.
- Reconcile conflicts by version, date, route, effort and harness; keep an unresolved
  disagreement in the snapshot instead of averaging it.
- `?` means insufficient evidence, never "average": don't turn it into `~` or `-`.
- COST stays anchored at luna = 1 and astra = 100 as a policy scale. When the measured astra
  ratio differs a lot, record it in the snapshot and ask the user whether to re-anchor.
  User-stated ratios (e.g. opus ≈ 2× sonnet) outrank list prices.
- Spot-check the sources behind every value that changes a spec rating; send only disputed
  or consequential claims to a stronger reviewer (e.g. one `codex:terra` turn), not the
  whole ledger.

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
for r in gpt6 gpt56 claude gemini price; do harnez agent delete --name mr-$r -d "$d"; done
rm -rf "$d"
```

## Not part of this plan

Running models on known-answer tasks (skill canaries) or the repo-graded advisory eval
([practices/ModelRoles.md](practices/ModelRoles.md) "Evaluating models") gives stronger
evidence but costs a turn per model; use them only when web research can't settle a role.
The cheap, worthwhile trials are planned in [ModelTrials.md](ModelTrials.md).
