# 573 — harnez bench: batch vs sequential read prompts and cached vs new input tokens

**Status**: Open
**Priority**: P2
**Severity**: Medium
**Category**: Bench / Tooling
**Related**: [[570-harnez-bench-read-and-summarise-lang-docs-task-with-keyword-checks]], [[562-harnez-bench-agy-main-vs-helper-model-split-read-benchmark]]

## Background

`read-lang-summary` on agy native/text used 185k–233k input tokens for six docs of ~17k tokens.
The per-call context series (run 191, agy:flash38:low native) is 12.5k → 29.4k over 9 calls,
growing by about one doc per call: the agent reads one file per round, and "input" sums the full
context of every call. Most of that repeated context is likely served from the provider cache, so
the number overstates real cost, and it mixes up two behaviours: batched vs sequential reads.

All three parsers already see cache data (agy meter `cachedContentTokenCount`, claude
`cache_read_input_tokens`/`cache_creation_input_tokens`, codex `cache_read_tokens`) but add it
into `InputTokens`.

## M1 — cached vs new input, batch vs sequential prompts

- Store cached input tokens per run (new column) next to `InputTokens`, from each provider's own
  field; keep `InputTokens` as the total. Where a provider reports no cache field, estimate cached
  as the sum of each call's previous context (per-call series), and mark the value as estimated.
  If neither is available, show `-`, not 0.
- Final `bench run` table: add `cached` and `new` (= input − cached) columns.
- Read-order variants for read tasks, as spec data (no prompt text in Go): e.g. a `read_order`
  option `batch` / `sequential` whose sentences come from `tasks.yaml`, appended to the per-mode
  prompt ("Read all files in one batch" / "Read the files one after another"). Selectable like
  `--read`, e.g. `--order batch,sequential`, expanding the matrix; default: no sentence (today's
  behaviour). Show the order in the table and the preamble.
- Unit tests with fake runners/usage: cached from provider field, estimate from the call series,
  `-` when unknown, matrix expansion with order, prompt sentence per order.
- Update `docs/Bench.md`. No live model calls. One `make test-q1`, commit `(issue 573 M1)`.

## M1 delivered (2b656ed)

`--order batch,sequential` with sentences in `tasks.yaml`, `order`/`cached`/`new` columns, `*` for
estimates, `-` when unknown. Codex (`input_tokens` includes cached) and agy (meter `Cached` of the
main model) are right. Dev q1 green.

## M2 — Pre-Work / Required Refinements

- `ParseClaude`: `cache_creation_input_tokens` is counted as cached. Cache creation is newly
  written input (billed above normal input), so it belongs in `new`; `cached` is
  `cache_read_input_tokens` only. Test: read 100 + creation 50 + input 10 → input 160, cached 100.
- `runner.go` `mainUsageModel` repeats the main-model pick of `splitAgyUsage`; reuse that instead.
- One `make test-q1`, commit `fix(bench): ... (issue 573 M2)` only if green.
