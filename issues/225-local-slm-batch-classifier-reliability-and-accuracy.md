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

With `qwen2.5-3b-instruct-q4` (the original default, picked only because it happened to be cached —
not benchmarked), roughly 60% of 20-note batches failed on category-count mismatch (model returned
17-21 categories for a 20-note batch), and spot-checking the categories it *did* return surfaced
plausible-looking but wrong classifications: generic/ambiguous notes such as `"clean success"` were
labeled `edit` — a category error rather than a `CategoryOther` fallback, so it isn't caught by any
existing safety net. Because such generic notes recur across many tool calls, one bad classification
for a distinct note text propagates to every call sharing that note, at scale (≈1700-1900 calls
landed under `edit` across two test exports, vs. 138 under Tier 1 alone).

## 2. Model Swap: qwen2.5-3b → qwen3-4b-instruct-2507-q4

`qwen2.5-3b-instruct-q4` was never chosen for quality — it was just one of three models issue 216's
problem statement happened to list as cached. A web search (2026-09-03) plus a check of
`~/projects/lmcoder/spec/models.yaml` confirmed `qwen3-4b-instruct-2507-q4` (Qwen3, July 2025) is
already cached on this box, a full model generation newer, similar size/speed, and has an 8x larger
native context (262144 vs 32768). `defaultClassifierModel` in `internal/telemetry/classify.go` was
switched to it, and the production default `Timeout` bumped from 10s to 45s (a 20-note batch against
qwen3-4b measured ~11s under light load; the old 10s default was tuned for qwen2.5-3b and became a
silent-failure trap against the slower, better model).

Re-running the same live diagnostic with qwen3-4b (batch size still 20, after **clearing
`note_category_cache`** — a stale-cache gotcha found along the way, see §3):

- Count-mismatch failures dropped from ~60% of batches to ~3 of ~13-14 batches per full export (still
  non-zero — see §4).
- `"clean success"` now correctly classifies as `other`.
- A full clean-cache export landed `edit` at 151 calls (vs. 138 under Tier 1 alone, vs. 1700-1900
  with the qwen2.5-3b regression) — i.e. no runaway category; the distribution (`test` 94, `build`
  92, `workflow` 53, `debug` 38, `git` 34, `config` 7, `inspection` 107) looks plausible on
  inspection, though not independently verified against ground truth.

This is a real improvement, not a full fix — see §4 for what's still open.

## 3. Stale-Cache Gotcha (fixed as a one-off, not a code change)

`note_category_cache` (Tier 2) has no invalidation on model/prompt-version change. Debugging with
qwen2.5-3b wrote ~193 entries into the real `~/.harnez/tool_catalog.sqlite`, some wrong (e.g.
`"clean success" → edit`); because Tier 2 is checked before Tier 3 ever runs again, those bad
entries stayed permanently stuck even after switching to qwen3-4b, until the cache was manually
cleared (`DELETE FROM note_category_cache`, done once by hand with the user's explicit go-ahead —
not scripted, since it mutates real production telemetry data). This is a latent gap: nothing
currently invalidates cached categories when `defaultClassifierModel`, `classifyPromptTemplate`, or
the classifier implementation itself changes. Worth a schema addition (e.g. a `classifier_version`
column) in a future pass if the model/prompt is expected to keep evolving.

## 5. Redesign: One Growing Session Instead of Batched JSON Arrays

The user's suggestion (2026-09-04): instead of asking for a whole batch's categories back as one
JSON array in a single request (the design that produced the ~20-60% count-mismatch failures
above), hold one conversation per chunk — a system preamble once, then one note per turn
(`"<n>: <note>"` → `"<n>: <category>"`), each turn appended to the message history resent on the
next call.

This eliminates the failure mode at its root rather than mitigating it: there is no array to close
early or lose count of, so a bad reply costs only its own note (falls back to `CategoryOther`
locally, session continues) instead of invalidating a whole chunk. Implemented in
`DefaultLocalClassifier.ClassifyBatch` (`internal/telemetry/classify.go`); `batchSize` (now the
conversation length before starting a fresh session, not an array size) raised from 20 to 60,
since large batches are no longer risky — only bounded by the local model's context window.
`classify_live_test.go`'s `TestDefaultLocalClassifier_LiveManual` verified a full 60-note session
against the real DB and a live server: **zero format failures**, all 60 notes classified, ~1.3s/turn
average (consistent with llama-server's prompt-prefix caching — confirmed earlier via its response
`cached_tokens` field — keeping each turn's reprocessing to just the newly appended note/reply
rather than the whole growing history).

A subsequent full `--classify` export (262 distinct Tier-1 misses, existing 182 cache entries left
in place) completed with **zero stderr warnings** — every batch's session ran to completion cleanly.
Resulting distribution: `other` 3045, `edit` 159, `inspection` 131, `test` 104, `build` 99,
`workflow` 61, `git` 54, `debug` 42, `config` 8 (total 3703 calls) — no runaway category, `edit`
back in the same range as the Tier-1-only baseline (138) despite classifying far more notes.

This resolves the reliability half of this ticket (§1-§2's count-mismatch problem). Accuracy
(whether individual classifications are *correct*, not just well-formed) is still not independently
verified against ground truth — see §6.

## 6. Possible Directions (not yet decided)

- Independently verify a sample of classifications against ground truth (have a human or a stronger
  model re-label a random sample, compare) — nothing here has measured *accuracy*, only format
  reliability and absence of runaway categories.
- Few-shot examples in the system preamble for ambiguous/generic notes (e.g. `"clean success"`,
  which has landed as `edit`, `build`, and `other` across different sessions/models in testing —
  still not obviously stable for the most generic notes).
- A `classifier_version`/model-tag column on `note_category_cache` so a model, prompt, or protocol
  change (like this session's array→session redesign) invalidates only the entries it should,
  instead of requiring a manual full clear (see §3 — this bit us twice in one afternoon).
- An even larger cached-on-demand model (`mistral-nemo-12b-instruct-q4`, `qwen3.8-27b-instruct-q4`)
  traded against latency and first-run download size — neither is cached yet on this box.
- Now that per-note failures are cheap and isolated, consider whether `batchSize` (session length)
  can grow further before hitting the context-window ceiling, trading fewer session restarts
  (each restart resends the system preamble cold) against a single very long session's risk of
  quality drift over many turns — not measured here.

## 7. Acceptance Criteria

Not yet scoped — file first, decide direction in a follow-up pass. At minimum:
1. Document (in this ticket or a `docs/studies/` note) a measured *accuracy* baseline (not just
   format-reliability, which §5 now covers) for the current model + protocol, ideally against a
   hand-labeled sample, so future changes can be compared against it.
2. Decide whether the current behavior (silent-but-logged fallback, occasional plausible-but-wrong
   labels, unverified accuracy) is acceptable for `activity_category`'s stated use as "safe for
   public visual analytics," given 212's premise that the taxonomy is supposed to be safe/low-risk
   even when wrong — or whether accuracy needs independent verification before `--classify` should
   be recommended for general use.
