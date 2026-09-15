# 363 — Dense dos/don'ts-only rewrite of lite docs, validated by canary-lite-doc

**Status**: Closed — dense rewrite shipped, reviewed, canary harness hardened to Go along the way
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

- [x] Rewritten `Bash.lite.md`/`Make.lite.md` drafted. Bash: 6237 -> 4767 bytes (-23.6% vs. prior
      lite, -51.3% vs. full doc). Make: 4724 -> 3998 bytes (-15.4% vs. prior lite, -26.8% vs. full).
- [x] 6 total fixtures (2 existing + 4 new: `bash-local-exitcode`, `bash-log-pipeline`,
      `make-deploy-parity`, `make-phony-help`) all pass against the rewrite.
- [x] Side-by-side comparison recorded in `scripts/canary-lite-doc/results.md`.
- [x] 0 lint findings on every fixture's generated output.
- [x] No fixture regressed — every rule transmitted correctly at the denser wording on the first
      run; independent review spot-checked 19 distinct rules across both docs with no omissions.
      One deliberate non-compression content change made along the way (not a regression): the
      awk/mawk portability appendix was made optional/on-demand (a one-line pointer to the full
      doc) rather than inlined, per explicit follow-up request — awk is rarely written in this
      codebase, so it no longer pays its token cost every session.

Side effect: the canary harness itself (`scripts/canary-lite-doc/`) was rewritten from bash to Go
during this work — the bash version's `tail -1` parsing of the agent's chat reply for the output
path broke nondeterministically when the model's reply put commentary after the path instead of
before it. The Go version checks for the fixture's explicitly-named output path directly instead
of parsing any model output, and lints via a direct `internal/lint` import.
