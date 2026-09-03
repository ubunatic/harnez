# 225 — Local SLM Batch Classifier: Reliability & Accuracy Follow-Up

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Quality / Telemetry
**Related**: [216-switch-telemetry-classifier-from-claude-cli-to-lmcoder-local-slm.md](216-switch-telemetry-classifier-from-claude-cli-to-lmcoder-local-slm.md), `internal/telemetry/classify.go`

---

## 1. Context

While running `harnez usage export --classify` for the first time after issue 216 landed
(switching `DefaultLocalClassifier` from `claude -p` to a local `lmcoder`/llama-server HTTP
endpoint), a live diagnostic against the real `tool_catalog.sqlite` DB (see
`internal/telemetry/classify_live_test.go`, `TestDefaultLocalClassifier_LiveManual`, opt-in via
`RUN_LIVE_CLASSIFY_TEST=1`) surfaced three wiring bugs, all fixed in the same session:

1. No `max_tokens` cap on the chat-completions request — a batch of 50 notes let the model
   generate until it exhausted context, taking minutes and surfacing only as a misleading
   `context deadline exceeded`.
2. Production `batchSize` of 50 notes was too large for the small local model
   (`qwen2.5-3b-instruct-q4`): past ~20-30 notes it fell into a repetition loop, emitting `"other"`
   well beyond the requested count instead of closing the JSON array. Reduced to 20, and a `"]"`
   stop sequence added so generation halts as soon as the array closes rather than relying solely
   on `max_tokens`.
3. `ClassifyNotes` swallowed every Tier 3 batch error completely — a broken classifier and a
   working one looked identical in the export output. Now prints a `harnez: warning: ...` line to
   stderr on each failed batch.

## 2. Remaining Issue (this ticket)

With all three wiring bugs fixed, a full `--classify` export over ~260 distinct Tier-1-miss notes
still saw roughly 60% of 20-note batches fail on category-count mismatch (model returned 17-21
categories for a 20-note batch), falling back to `CategoryOther` for those batches — which is
graceful per issue 216's AC3, but means real coverage is well under 100% even when everything is
wired correctly.

More importantly, spot-checking the categories the model *did* successfully return surfaced
plausible-looking but wrong classifications: generic/ambiguous notes such as `"clean success"` were
labeled `edit` — a category error rather than a `CategoryOther` fallback, so it isn't caught by any
existing safety net. Because such generic notes recur across many tool calls, one bad
classification for a distinct note text propagates to every call sharing that note, at scale
(≈1700 calls landed under `edit` in one test export, vs. 138 under Tier 1 alone).

## 3. Possible Directions (not yet decided)

- Few-shot examples in `classifyPromptTemplate` for ambiguous/generic notes.
- A larger cached local model (`qwen3-4b-instruct-2507-q4` or `qwen2.5-3b` alternatives) traded
  against latency.
- Per-note (batch size 1) classification for notes below some ambiguity heuristic, at the cost of
  more requests.
- Treat count-mismatch and low-confidence responses as a stronger signal — e.g. retry once with a
  smaller sub-batch before falling back to `CategoryOther`, rather than discarding the whole batch.

## 4. Acceptance Criteria

Not yet scoped — file first, decide direction in a follow-up pass. At minimum:
1. Document (in this ticket or a `docs/studies/` note) a measured accuracy/coverage baseline for
   the current model + prompt, so future changes can be compared against it.
2. Decide whether the current behavior (silent-but-logged fallback, occasional wrong-but-plausible
   labels) is acceptable for `activity_category`'s stated use as "safe for public visual
   analytics," given 212's premise that the taxonomy is supposed to be safe/low-risk even when
   wrong — or whether accuracy needs to improve before `--classify` should be recommended for
   general use.
