# 363 — Dense dos/don'ts-only rewrite of lite docs, validated by canary-lite-doc

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Feature
**Category**: Templates / Docs / Token Efficiency
**Related**: [Issue 359](359-pilot-agenticloop-lite-md-behavioral-canary-gate.md) (pilot lite doc),
[Issue 362](362-llm-invocation-canary-harness-to-automate-lite-doc-behavioral-scoring.md) (shipped
the `scripts/canary-lite-doc/` harness this ticket's rewrite must pass), `docs/lang/Bash.lite.md`,
`docs/lang/Make.lite.md`

---

## 1. Problem & Motivation

The current `Bash.lite.md`/`Make.lite.md` (shipped 2026-09-15) already pass
`scripts/canary-lite-doc/run.sh`'s isolated-agent + `harnez lint --check` canary cleanly — see
`scripts/canary-lite-doc/results.md`. They were authored "code-first, comments carry the rules,"
but still keep full sentences/prose in code comments and markdown tables (e.g. "Rule: if the
command has a directory flag, use it — never `cd` purely for scoping. The shell tool's cwd
persists across tool calls...").

The user's request during that work: go further — keep only *meaning*, not words. A full lean
rewrite using terms, rules, math, code, priorities, dos/don'ts in the densest form deliverable, not
word-preserving compression of the existing prose. Since the current docs already pass the canary,
this ticket is about finding out whether an even denser rewrite still passes it, not about fixing a
known deficiency — the current lite docs are not broken.

## 2. Proposed Fix

1. Rewrite `Bash.lite.md` and `Make.lite.md` (and any other lite doc added later) into the densest
   form that still transmits every rule: symbolic/terse notation over sentences, dos/don'ts lists
   over paragraphs, code+inline-comment over markdown tables where a table doesn't add value beyond
   what a comment could. Explicitly not a word-count-preserving edit — a rewrite from scratch aimed
   at minimum tokens per rule.
2. Validate with `scripts/canary-lite-doc/run.sh` using the SAME fixtures already in
   `scripts/canary-lite-doc/fixtures/` (bash-deploy-check, make-widget) plus at least 2-3 new
   fixtures per doc that exercise rules the existing 2 fixtures don't reach (e.g. Bash's
   `local`-declare-then-assign nuance, Make's deployment-target-parity section — neither fixture
   above touches those).
3. Ship only if the denser rewrite's canary pass rate is >= the current lite docs' (currently
   100% on both fixtures) — same "ship only if it scores >= the baseline" gate issue 359
   established, now backed by a real harness instead of manual reasoning.
4. If the denser rewrite fails a fixture the current lite doc passes, that's a concrete signal
   that a specific rule needs more than a symbol/dos-donts line to transmit correctly — record
   which rule and why in this ticket's resolution rather than silently reverting.

## 3. Acceptance Criteria

- [ ] Rewritten `Bash.lite.md`/`Make.lite.md` drafted, targeting meaningfully smaller byte count
      than the current lite docs (report actual bytes; no fixed target — smaller than current is
      the bar, not a specific percentage).
- [ ] At least 4-6 total fixtures across both docs (existing 2 plus 2-4 new ones covering
      currently-untested rules) all pass `scripts/canary-lite-doc/run.sh` against the new rewrite.
- [ ] Side-by-side canary run comparing current lite docs vs. the rewrite on the same fixture set,
      recorded in `scripts/canary-lite-doc/results.md`.
- [ ] `harnez lint --check` on every fixture's generated output still reports zero findings (the
      mechanical judge, not a subjective read of "does this still transmit the rule").
- [ ] If any fixture regresses vs. the current lite doc, do not ship that section's rewrite — keep
      the current wording for that rule and document why in this ticket.
