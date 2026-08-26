---
title: ConciseMode Practices
weight: 46
---

# ConciseMode — Graded Output Terseness

Output generation latency, not reasoning, is the bottleneck on slow inference paths (e.g. local
LLMs at single-digit tokens/second). Conversational pleasantries, hedging, and verbose recaps
cost real wall-clock time without adding signal. ConciseMode defines three selectable tiers of
terseness an agent can be asked to operate under.

**Core invariant across all levels**: code, diffs, tool parameters, and command syntax are always
preserved 100% verbatim. ConciseMode trims narration, never content.

---

## Level 1 — Concise Lite (Professional Terse)

- Drop opening pleasantries (*"Certainly!", "I'll be happy to help..."*) and speculative closing
  remarks.
- Keep full grammar and complete-sentence explanations.
- Default tier for interactive user pairing — the reader still wants prose, just without filler.

## Level 2 — Concise Standard (Telegraphic / Core Caveman)

- Strip articles, connective phrases, and grammatical filler.
- Structure findings as dense fragments: `Action -> Finding -> Patch`.
- Use for autonomous subagents and fast canary/benchmark sweeps where no human reads the stream
  live.

## Level 3 — Concise Ultra (Extreme Shorthand / Zero-Fluff)

- Output confined to essential diffs, command invocations, and single-line status confirmations,
  e.g. `PASS: 14 tests, built bin/app`.
- Zero narrative text.
- Use only on resource-constrained or low-TPS local hardware where every output token has
  measurable latency cost.

---

## Pairing with Output Distillation

ConciseMode governs what the agent *writes*; `harnez distill` (see issue 066) governs what the
agent *reads back* from tool output. The two compound: distillation keeps noisy command output out
of context, ConciseMode keeps the agent's own narration short. Neither substitutes for the other —
apply both on constrained hardware.
