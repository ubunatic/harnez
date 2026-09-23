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
| `mr-price` | all in-scope models plus both anchors | COST and EFF on one scale |

Per-model researchers (2026-09-23 first sweep) produced ratings that didn't line up;
per-column researchers repeat the identity work for every column. Family researchers plus
one scale owner avoid both. Skip families outside the trigger's scope.

Run in parallel from a scratch dir outside the repo, on `codex:luna:med`, role advisor
(a web-only researcher role is issue 510). Write the prompts below to
`$d/prompt-<r>.txt` first, with `<rows>` replaced by the spec keys (e.g.
`codex:luna, codex:sol, codex:astra`); drop out-of-scope families from the loop. The
current table is appended to every prompt:

```bash
d=$(mktemp -d); harnez agent models > "$d/models.txt"   # keep $d until step 6
# write $d/prompt-{gpt6,gpt56,claude,gemini,price}.txt here
for r in gpt6 gpt56 claude gemini price; do
  harnez agent start --name mr-$r --role advisor --model codex:luna:med -d "$d" \
    --stream stats "$(cat "$d/prompt-$r.txt" "$d/models.txt")" > "$d/$r.txt" 2>&1 &
done
wait
```

Family prompt (`$d/prompt-<family>.txt`):

```text
WEB RESEARCH ONLY. Do not inspect any repository or local files.
The table below guides model choice for agentic coding. Audit rows: <rows>.
For each row: verify the model exists under that exact id, its version, provider route and
effort controls (what low/med change; whether EFFORT "no" means unsupported). Then audit
every SKILLS (Go, TUI, SQL; ignore Python, JS/TS, web), ROLES and USE claim: turn each
phrase into a checkable statement ("slow first token" = measured first-token latency) and
look for model-specific evidence. Never infer one route (e.g. agy vs claude) from another.
Output a ledger, one line per claim, no other text:
key | column | claim | verdict (confirmed/contradicted/?) | proposed value | evidence with
date | source URL | confidence (high/med/low)
key = spec key as in the table (e.g. codex:luna); column = ID, EFFORT, SKILLS-Go,
SKILLS-TUI, SKILLS-SQL, ROLES or USE. At most 40 lines.
```

Price and efficiency prompt (`prompt-price.txt`):

```text
WEB RESEARCH ONLY. Do not inspect any repository or local files.
For these models: <rows> (always including codex:luna and codex:astra):
COST: official input/output/cached/reasoning token prices; subscription price, quota and
reset rules and any overage. Price one fixed workload (100k input, 20k output, 50%
cached) per model and report it as a multiple of gpt-6-luna = 1. Report astra's measured
ratio; do not force it to 100. Keep cash price and quota consumption separate; do not
invent cross-vendor quota conversions.
EFF: tokens (including retries) per SUCCESSFUL task from a named harness and task suite
(e.g. Terminal-Bench or SWE-bench cost figures). Reject leaderboard scores or speed as a
substitute; "?" if no such measurement exists.
Output one line per model, no other text:
key | list price (USD/1M in/out/cached) | workload ×luna | quota notes | EFF (+/~/-/?) |
evidence with date | source URL | confidence (high/med/low)
key = spec key as in the table. At most 40 lines.
```

### 2b. Local quota analytics (one read-only turn, in parallel with step 2)

**Light touch:** this is a basic look at our own data to confirm or complement the web
research, not the main work. One turn, no tooling built, no deep dives; if the data is
messy, report what was tried and move on. Proper per-turn measurement is issue 507.

Web research gives list prices; our own quota history shows what a model really costs on
our subscriptions and projects. One analytics agent (`codex:luna:med` or `claude:sonnet`,
role advisor, run from the repo) correlates quota readings with agent turns:

| Source | Holds |
|---|---|
| `~/.claude/harnez/usage-history/quota-history.jsonl` | 5h and weekly `used_percent` per provider, ~3 min cadence (the live store; per-host `*.jsonl` there may be stale) |
| `~/.harnez/agents/*.json` | harnez agent sessions: `id`, provider, model, tier, role, cumulative tokens (cached split), `created_at`/`last_active_at` only; no per-turn times |
| `~/.codex/sessions/YYYY/MM/DD/rollout-*-<id>.jsonl` | Codex rollouts: `turn_context` (model, effort), `token_count` events with per-call tokens incl. reasoning and the account's `rate_limits` (primary = 5h, secondary = weekly) |
| `~/.claude/projects/*/*.jsonl` | Claude Code transcripts (host session and `claude:*` agents): every assistant message has a UTC `timestamp`, `message.model` and `message.usage` (input, output, cache read, cache write) |

agy has no per-turn token record found yet and its quota readings are sparse (~20–30 min);
treat agy numbers as rough session-level estimates.

Format traps (checked 2026-09-24):

- `window` labels differ per provider (`5-Hour`, `Session (5-hour)`, `Five Hour Limit
  Remaining`, `Weekly`, `Weekly (7-day)`, `Weekly Limit Remaining`); normalise to 5h/weekly.
  "Remaining" series count down: use `used_percent`, never `remaining_percent`.
- Quota and rollout event timestamps are UTC (`Z`); agent records carry a local offset
  (`+02:00`); rollout file names use local time. Convert everything to UTC first.
- A window reset (`reset_at` changes) drops `used_percent`; never read it as a negative step.
- Some rollout `token_count` events have `rate_limits.primary`/`secondary` = null; skip
  those readings instead of treating them as 0.
- Deleted agent records (step 6) leave rollouts without a model mapping; use the
  rollout's own `turn_context.model` instead.
- The host session runs on Claude and uses Claude quota continuously, so intervals with
  only one Claude agent active are rare; its messages are in the same transcripts.

Until issue 515 gives one discovery entry point, pass these paths explicitly; the empty
`~/.harnez/telemetry.sqlite` and `~/.local/share/harnez/telemetry.db` are not the data.

Prompt:

```text
Read-only analytics over local files; no web search, no edits. Sources: <paths above>.
Normalise all timestamps to UTC and window labels to 5h/weekly (see format traps).
Codex: use rollout token_count events; each carries the account-wide used_percent, so a
step between two events of one rollout is attributable only if no other rollout has
token_count events in that interval.
Claude: don't wait for clean intervals. Sum transcript usage per model (all projects,
host session included) between consecutive quota readings and fit the 5h used_percent
steps against those per-model token sums (simple least squares, weights per model).
agy: session-level estimate only (agent-record time span vs quota readings).
Skip intervals with a window reset or unrecorded activity, and say how many you skipped.
Report per model:
turns, new/cached/reasoning tokens, quota points consumed, points per 100k new tokens,
and a cost multiple vs gpt-6-luna within the same provider. Mark every estimate with
its sample size and rounding caveat (used_percent is an integer). List clean
before/after pairs as evidence (e.g. 2026-09-23 21:37→21:51 UTC: one gpt-6-astra turn,
18.7k new tokens, 5h 0→2%, while 11 earlier luna turns left it at 0%).
Output: one table row per model plus at most 10 lines of findings.
```

Measured numbers outrank list prices for COST and EFF within a provider; cross-provider
multiples stay list-price based. Run it with the step 2 loop's `harnez agent start`
(`--name mr-quota`, `-d .` from the repo root). Needs session records: run this before step 6, and
don't delete `~/.harnez/agents` records of research runs you may want to analyse later.

### 3. Reconcile

- Source ranking: official pricing and model docs for facts, reproducible independent
  evaluations for capability; marketing, anecdotes, affiliate posts and unsourced rankings
  are leads only.
- Reconcile conflicts by version, date, route, effort and harness; keep an unresolved
  disagreement in the snapshot instead of averaging it.
- `?` means insufficient evidence, never "average": don't turn it into `~` or `-`.
- COST/EFF precedence: user-stated ratios (e.g. opus ≈ 2× sonnet) > measured trials and
  quota (step 2b, [ModelTrials.md](ModelTrials.md); within one provider only) > list
  prices (step 2). Cross-provider multiples stay list-price based.
- COST stays anchored at luna = 1 and astra = 100 as a policy scale. Ask the user about
  re-anchoring only when a measured astra ratio falls outside 50–200 (2× off) on more than
  one clean measurement; record every measurement in the snapshot either way.
- Merge ledgers by `key | column`; a cell changes only on a `contradicted` verdict with
  med/high confidence, else it stays (or becomes `?` if nothing supports it).
- Spot-check the sources behind every value that changes a spec rating; send only disputed
  or consequential claims to a stronger reviewer (e.g. one `codex:terra` turn), not the
  whole ledger.

### 3b. Each agent writes a short study

More important than the numbers: every researcher (step 2) and the analytics agent (step
2b) writes its own short study, so the findings and methods survive the session.

- One file per agent: `docs/studies/<YYYY-MM-DD>-model-research-<r>.md` (e.g.
  `...-model-research-gpt6.md`, `...-model-research-quota.md`).
- Concise: at most ~40 lines. Question, method (sources, queries), findings with source
  URLs or data paths, what was inconclusive, what to try next time.
- Separate files, no merging; the host links them from the dated [Models.md](Models.md)
  snapshot instead of copying them.
- Add to every prompt: "Also write your findings as a short study (≤40 lines) to
  <path>; that file is the only file you may create." Research agents run from `$d`, so
  give the absolute repo path, and the host commits the studies with the snapshot.

### 4. Update spec and snapshot

- Edit `cost`, `eff`, `skills` (and `use`/`roles` if the evidence changes a role) in
  `spec/agent.yaml`; keep the header comment and `models_legend` in sync if a scale changes.
- Add a dated `## YYYY-MM-DD <topic> snapshot` section to [Models.md](Models.md): scope,
  table, findings, research cost, links to the step 3b studies. Never rewrite older
  snapshots.

### 5. Check and commit

```bash
make install
script -qc 'harnez agent models' /dev/null            # real pseudo-terminal
make test-q1 > "$d/q1.log" 2>&1; grep -- '--- FAIL' "$d/q1.log"
```

Commit spec, snapshot and studies together, e.g. `feat(agent): <date> model research refresh`.

### 6. Cleanup

```bash
for r in gpt6 gpt56 claude gemini price; do harnez agent delete --name mr-$r -d "$d"; done
harnez agent delete --name mr-quota -d .
rm -rf "$d"
```

## Not part of this plan

Running models on known-answer tasks (skill canaries) or the repo-graded advisory eval
([practices/ModelRoles.md](practices/ModelRoles.md) "Evaluating models") gives stronger
evidence but costs a turn per model; use them only when web research can't settle a role.
The cheap, worthwhile trials are planned in [ModelTrials.md](ModelTrials.md).
