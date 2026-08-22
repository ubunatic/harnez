# 045 — Multi-round review loops harden the symptom, not the root cause

**Status**: Open
**Category**: Agentic Ergonomics / Review Practice
**Related**: [docs/practices/AgenticLoop.md](../docs/practices/AgenticLoop.md), [039](039-agentic-loop-practices-and-sprint-command.md)

---

## 1. Problem & Motivation

Observed in a sibling harnez-managed project (`weg`), on a real ticket
(installing Nextcloud over SSH on a shared webspace): a JSON status check
(`occ status --output=json`) intermittently had a PHP diagnostic notice
printed ahead of the real JSON on stdout. This was treated purely as a
parsing-robustness problem and iterated on across **four separate rounds**:
each fresh-Opus review found a new adversarial input that broke the current
parsing heuristic (a nested JSON object, a brace inside a string value, a
trailing object, a same-shape-but-wrong object one level deeper), and each
round's fix made the candidate-selection logic more defensive rather than
questioning the premise.

**Nobody — not the dev agent (two separate rounds), not any of the four
independent fresh-Opus reviewers, not the orchestrating session — asked "why
is a diagnostic notice in stdout at all" until a human asked the coordinator
directly, in a completely different framing** ("does PHP not decide between
stdout and stderr? I'm confused why such a standard tool would not split the
output"). Every agent involved, across five independent invocations with
fresh context, stayed inside the frame the prior round set: "make the parser
survive this input" rather than "why does this input exist."

**Second layer, worth recording because it strengthens rather than weakens
the finding**: the coordinator's own answer to that human question — "PHP's
`display_errors` ini setting defaults to routing CLI notices to stdout, set
it to `stderr`, done" — was itself a plausible-sounding, unverified theory,
and it turned out to be *wrong* for this specific case. When a follow-up
ticket went to implement it, `-d display_errors=stderr` did not suppress
the notice. The actual mechanism (found only by extracting and reading the
real third-party source): the CLI tool in question prints that notice via a
plain `echo` gated by its own `--no-warnings` argv flag — never routed
through PHP's error handler at all, so no `display_errors` setting could
ever have touched it. `--no-warnings` was the real fix.

So the root-cause-vs-symptom gap isn't limited to review passes narrowly
scoped to "does this fix work" — it also caught the *human-prompted,
reframing* question itself, whose answer still went unverified for a beat
before being written into docs and this very ticket as settled fact. The
practice gap this ticket proposes (ask "is there a simpler upstream fix,"
and *verify it before treating it as the fix*) would have caught both
layers, not just the first.

## 2. Detailed Technical Specification

This is a review-practice gap, not a one-off missed fact. The pattern:

- A review is scoped to "does this fix correctly solve the reported
  problem" — which is the right question for verifying a fix, but it
  implicitly inherits the previous round's framing of *what the problem is*.
- A fresh reviewer with no memory of prior rounds still inherits the framing
  via the diff/ticket they're handed: "here's a parsing bug, here's the fix,
  is the fix correct" primes for "is this fix correct," not "is this the
  right fix."
- Nothing in the current review loop practice (`docs/practices/AgenticLoop.md`
  Phase 3, or equivalent review-agent prompting guidance) asks a reviewer to
  step back and consider whether a defensive/reactive fix has a simpler
  upstream alternative that would eliminate the need for the defense
  entirely — especially relevant for classes of bug that are "tool output is
  noisier than expected" (a very common shape: CLI tools, log scraping,
  subprocess output parsing in general).

## 3. Implementation & Verification Plan

Documentation-only change, no code:

1. Add explicit guidance to `docs/practices/AgenticLoop.md`'s Phase 3
   (Pre-Commit Review Gate) checklist: alongside "does this fix work," ask
   "is there a simpler upstream fix (at the source of the noisy/unexpected
   input) that would make this defensive code unnecessary?" — particularly
   for parsing/robustness fixes against subprocess or tool output.
2. Consider a standing prompt addition for review-agent invocations
   specifically: when reviewing a fix to defensive parsing/error-handling
   code, explicitly ask the reviewer to identify the upstream source of the
   unexpected input and assess whether *that* can be fixed instead of (or
   in addition to) hardening the consumer — and, per the second layer found
   above, to actually verify a proposed upstream fix against the real
   source/tool before it's treated as the fix, not just accept a
   plausible-sounding mechanism on its face.
3. No test/verification beyond re-reading the doc for consistency — this is
   guidance, not enforced logic.
