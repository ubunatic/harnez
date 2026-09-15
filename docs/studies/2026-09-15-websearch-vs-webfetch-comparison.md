<!-- harnez:topic: side-by-side comparison of Claude Code's WebSearch and WebFetch tools via two isolated subagents fetching/searching the same real target (harnez.org/tools/voxi), covering accuracy, token cost, and when each is the right choice -->

# `WebSearch` vs `WebFetch`: a Side-by-Side Comparison

**Scope**: Two isolated fresh subagents were dispatched with the same target — summarize
"voxi" from `harnez.org/tools/voxi` — one restricted to `WebFetch` only, one restricted
to `WebSearch` only. Neither could see the other's work. Both then self-reported their
token cost and confirmed which tool(s) they used.

**Accessed**: 2026-09-15

## Setup

- **Agent A** (WebFetch): `WebFetch(url="https://harnez.org/tools/voxi", prompt="Summarize what voxi is according to this page.")`, one call.
- **Agent B** (WebSearch): `WebSearch(query="voxi harnez.org")`, one call.

Neither agent was told the correct answer in advance or given the other's result.

## Result 1 — accuracy: `WebFetch` was correct, `WebSearch` was wrong

**Agent A (WebFetch)** — correct, direct, and specific:

> voxi is a speech-to-text dictation tool for Linux/Wayland systems. It transcribes
> speech into text within the currently active application, with all processing done
> locally on-device (no cloud). Features include a keyboard-shortcut toggle for voice
> input, customizable text-replacement rules, and optional physical modifier-key
> support. Requirements: Go 1.26.5+, audio capture, and clipboard access.

**Agent B (WebSearch)** — wrong, and it flagged its own uncertainty correctly:

> Search results for "voxi harnez.org" did not return a direct harnez.org page about
> voxi. The only description surfaced (via the search tool's own synthesized summary,
> not a clear on-domain snippet) characterizes voxi as a visual analytics dashboard for
> AI coding sessions — tracking tool-call frequency, failure rates, quality scores, and
> token efficiency over time — positioned as a complementary tool to harnez... top-ranked
> hits were mostly unrelated (Geoengineer.org's unrelated "VOXI" software, Wikipedia
> "Vox" pages, harnez GitHub/Codeberg repo pages), so confidence in this voxi description
> is low/uncertain.

This is not a minor discrepancy — the two descriptions describe entirely different
products (a local speech-to-text dictation tool vs. a coding-session analytics
dashboard). `WebSearch`'s own synthesized summary produced a specific, confident-sounding
answer that was simply incorrect, though to its credit the subagent itself flagged low
confidence rather than presenting it as fact.

## Result 2 — token cost: `WebSearch` used more tokens for a worse answer

Both agents self-estimated the incremental cost of their one web call (not their whole
session — see caveat below):

| | Agent A (WebFetch) | Agent B (WebSearch) |
|---|---|---|
| Self-estimated call cost | ~200-250 tokens (URL+prompt ~30 in, ~150-200 result) | ~300-500 tokens (schema ~250 + query/results/summary/sources ~300-500) |
| Result quality | Correct, specific | Wrong, low-confidence |
| Harness-reported total subagent tokens (whole session) | 57,006 | 57,876 |

The whole-session totals are dominated by fixed subagent overhead (system prompt, tool
definitions, the agent's own reasoning) and aren't a clean measure of the tool call
itself — both agents landed within ~1.5% of each other on total tokens because that
overhead swamps a single tool call's cost. The self-estimated *incremental* call cost is
the more meaningful number here, and `WebSearch` came in higher: fetching one full page
you already have the URL for is cheaper than a search round-trip carrying multiple
title/URL/snippet entries plus a synthesized summary and a sources reminder.

## Takeaway: when to use which

- **You have a URL already** (a link the user gave you, a known docs page, an issue/PR
  URL): use `WebFetch`. It was both cheaper and correct in this test — reading a specific
  page beats guessing which search result is the real one.
- **You don't have a URL and need to find one** (unknown project name, "what's the
  current best library for X"): `WebSearch` is the only option, but treat its synthesized
  summary as a lead to verify, not a fact — in this test it confidently produced a wrong
  answer by conflating unrelated results (a same-named but unrelated product, a
  Wikipedia disambiguation page, and the wrong project's own repo pages). The right
  pattern is `WebSearch` to find candidate URLs, then `WebFetch` the most likely one to
  confirm — not `WebSearch` alone as a final answer.
- Neither tool is a `--debloat` candidate on the strength of this test: `WebFetch`
  performed better on both axes, so denying it to save `WebSearch`'s marginally higher
  cost would trade away the *more* reliable of the two tools, not the less useful one
  (see the earlier assessment in issue 316, which put both in the "ambiguous — ask before
  including" bucket for that reason).

## Method note

This comparison used two single-shot subagent dispatches rather than a fork or a
teammate, specifically so neither agent's tool choice or result could bias the other
(they ran with zero shared context). Each was told explicitly to use only one tool family
and to confirm which tool(s) it actually called — worth doing whenever a comparison's
credibility depends on the two sides not being able to see or influence each other.
