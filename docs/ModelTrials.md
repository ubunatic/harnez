# Model Trials

Cheap, repo-local trials that settle what web research ([ModelResearch.md](ModelResearch.md))
can't: Go, TUI and SQL skills (no model-specific benchmarks exist), EFF (tokens per goal)
and role claims in `harnez agent models`. Ideas from a `codex:astra` design review
(2026-09-23), cut to what is worth its cost. Status: plan, not yet run; no trial fixtures
exist yet.

## When to run

Only when a web-research snapshot leaves a consequential cell at `?` or disputed, e.g. a
model is a candidate for a role but its skill or EFF is unknown. Never as a routine sweep.

## Budget rules

- One task, one turn, one model at its configured tier. Rerun (n = 2) only when the result
  is borderline; no large samples.
- Try cheapest first: run COST ≤ 16 models freely, COST 20–32 only as candidates for a
  role, astra (COST 100) only on explicit request.
- Compare tokens only within one vendor; token counts are not comparable across tokenizers.
- A full skill round (3 tasks × 5 cheap models) is ~15 short turns, mostly cached input.

## Trials worth running

| Trial | Measures | Cost | Value |
|---|---|---|---|
| Skill canaries | SKILLS Go / TUI / SQL | 1 turn per task and model | high: the only real evidence for these columns |
| EFF from the same runs | tokens per accepted goal (retries included) | free: read `--stream stats` totals | high: fills the mostly-`?` EFF column |
| Honesty checks on the same diffs | test weakening, unasked edits | free: diff review | high: the known cheap-model failure modes |
| Seeded-bug review | reviewer role: bugs found / false alarms | 1 read-only turn per model | high: backs every `reviewer` role claim |
| First-token latency | "slow first token" (flash38) | free: stream timing | low, but costs nothing |

### Skill canaries

Three fixed tasks with a known answer and a machine-checkable acceptance test, kept in the
repo so every run is comparable:

- **Go**: a small bug with a failing test; pass = the test goes green without changing its
  assertions.
- **TUI**: a table or status line with wide runes (CJK, emoji) that must align by display
  width; pass = a golden-output test.
- **SQLite**: a known schema and data, three questions with known answers, one of them a
  fan-out join that double-counts unless aggregated first; pass = exact result sets.

Score: pass/fail, tokens used, and whether the diff touched only what the task allowed.
Map to ratings with fixed thresholds decided before the run (e.g. `+` passes first try,
`~` passes after one follow-up, `-` fails), and record the thresholds in the snapshot.

### Seeded-bug review

One diff with three planted defects (a weakened assertion, an off-by-one, a leaked
environment variable) and one harmless change. Ask each reviewer candidate for findings by
severity. Score: defects found, false alarms. This tests the `reviewer` role directly.

## Not worth it

- Blinded scoring and repeated-run statistics: too many turns for a solo repo.
- Orchestrator coordination trials: a full sprint per model.
- Cross-vendor token normalisation: tokenizers differ; compare within a vendor only.
- Controlled latency benchmarks beyond stream timing.

## Results

Record trial results as a dated snapshot in [Models.md](Models.md) next to the web research,
marked as measured, and update `spec/agent.yaml` from them; measured results outrank web
evidence.

Open: the fixtures (tasks, golden files, seeded diff) and a runner don't exist yet;
building them is its own ticket.
