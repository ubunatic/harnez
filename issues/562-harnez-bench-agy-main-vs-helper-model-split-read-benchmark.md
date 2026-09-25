# 562 — harnez bench: agy main vs helper model split, read benchmark

**Status**: Open
**Priority**: P2
**Severity**: Medium
**Category**: Bench / Tooling
**Related**: [[561-harnez-bench-cleanup-agy-provider-generic-bench-md]], [[563-meter-claude-and-codex-traffic-like-agy-proxy-incl-hidden-helper-calls]]

## Background

The agy meter shows that agy sends, per user prompt, extra small requests to a helper model
(`gemini-3.5-flash-lite`, ~94 input tokens) besides the selected model (e.g.
`gemini-3.7-flash-low`). Both carry the same prompt ID. `harnez bench` (issue 561 M3) currently
adds all meter rows into input tokens and turns, which inflates turns and makes a per-call
context curve jump.

Other agent CLIs (claude, codex) likely make similar hidden calls (titles, summaries); we cannot
see them without a proxy (issue 563). So the benchmark measures the **main session only**, and
keeps helper calls as extra data where we have them.

## M1 — split main and helper meter rows

- Group a run's meter rows by the `model` reported in the response. The main model is the one
  with the largest `promptTokenCount` over the run (no name mapping between CLI flags and meter
  model names).
- Main rows drive `InputTokens`, `TotalTokens`, `Turns` and a per-call context series (prompt
  tokens per main call, in order), stored with the run.
- Other models are stored as helper data: calls, input and total tokens, per model. Shown in
  `harnez bench results` as a separate column, never added into the main numbers.
- Unit tests with fake meter rows: main + helper, helper only between main calls, single model.
- No live runs in this milestone. One `make test-q1`, commit `(issue 562 M1)`.

### M1 delivered (7165ff5)

`splitAgyUsage` picks the model with the most prompt tokens as main; helpers stored per model
and shown in results. Host: diff OK, make test-q1 green, installed. Leftover:
`aggregateAgyUsage` is now unused; remove it with the next code change.

## M2 — dropped (2026-09-25)

The user runs small comparisons by hand instead of an 18-run sweep; see 565.

## M2 (dropped) — read benchmark run

- Conditions: `native`, `text`, `card` on agy `flash37` (low effort).
- Fixture sizes: ~100, ~400, ~1,000 lines; 2 repeats each (18 runs).
- Report per condition: pass rate, main input tokens, context growth per call, helper tokens,
  peak harnez RSS during `card` runs (543 guard).
- Real cost: record agy quota fractions (meter quota rows, `harnez usage`) before and after the
  sweep and report the quota drained next to the token totals.
- Stop after the first `card` run if its context did not grow by about the card's estimated
  tokens or the answer failed (model did not see the image).
- Results go into a study under `docs/studies/`, not into `docs/Bench.md`.
