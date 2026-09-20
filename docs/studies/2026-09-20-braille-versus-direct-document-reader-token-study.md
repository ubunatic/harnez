---
title: Braille versus Direct Document Reader Token Study
---

<!-- harnez:topic: Braille-mediated versus direct-Markdown subagent reading cost -->

**Date:** 2026-09-20  
**Scope:** Compare two `gpt-5.6-luna` low-reasoning subagent sessions summarizing repository documentation.  
**Goal:** Measure whether Dot8/Braille input changes reader-agent cost or reliability compared with original Markdown.

## Executive Summary

The Braille reader used session `01a0be86-…`; the direct-Markdown control used `01a0bedd-…`. Both produced coherent summaries. The Braille session initially had one dot-7/dot-8 decoding mistake, corrected on retry. It also processed more documents, so aggregate totals do not isolate representation cost.

| Reader | Calls | Recorded tokens | Input | Output |
|---|---:|---:|---:|---:|
| Braille | 38 | 1,071,904 | 544,426 | 3,990 |
| Direct Markdown | 17 | 483,276 | 241,995 | 3,281 |

## What Worked Well

- Reusable low-cost reader sessions made the comparison operationally consistent.
- Prompts restricted each reader to one target document at a time.
- The Braille reader decoded ordinary prose and Markdown structure after correcting its decoder.
- Distinct session IDs enabled post-hoc telemetry comparison.

## Honest Post-Mortem (Failures, Bugs & Near-Misses)

- The Braille reader initially inverted dot 7 and dot 8. It retried with the corrected mapping and obtained coherent output.
- The workloads were unbalanced: Braille reached roughly 35 documents before interruption; direct Markdown processed only part of the list.
- `harnez stats` has no filename/prompt field, so it cannot attribute tokens to individual documents.
- `total_tokens` are repeated context snapshots on tool calls, not exact provider billing.

## Quality & Invariants Audit

| Area | Result | Evidence |
|---|---|---|
| Session separation | Pass | Two distinct Codex session IDs. |
| One-document prompt boundary | Pass | Prompts named one file and prohibited other targets. |
| Braille readability | Conditional pass | One decoder correction was needed. |
| Direct control | Pass | Original Markdown only. |
| Per-file accounting | Gap | No filename/prompt field in telemetry. |
| Balanced experiment | Gap | Different file counts and stopping points. |

## Efficiency & Velocity Assessment

The Braille session averaged about 28,208 recorded tokens per call; direct Markdown averaged about 28,428. Per-call telemetry was nearly identical. The 2.22× total difference came primarily from the 38 versus 17 calls, not evidence that Braille itself costs 2.22× more.

A fair experiment must send the same document set to both readers and record a stable document ID with every prompt and completion.

## Key Learnings & Evergreen Upstream

- Compare per-document input, output, elapsed time, retries, and issue count; treat session totals as secondary context.
- Persist `doc_id` with subagent prompts or tool-call records.
- Record decoding issues separately from source-document issues.
- Keep the Dot8 rulebook available while forbidding original Markdown during representation-fidelity tests.

## File & Diff Summary

- Added this study document.
- Updated `docs/README.md` to index it.
- No code changes or commit were made for this study; unrelated working-tree changes were preserved.
