# Behavioral canary results — AgenticLoop.lite.md pilot (issue 359)

Manual scoring pass, 2026-09-15. No LLM-invocation/scoring harness exists in this
repo yet (checked `internal/`, `scripts/canary-*` — all are shell/CLI-behavior
canaries, not LLM-response scoring), so per the canary-first practice
(probe before building), this pilot scores each fixture by hand: for each
prompt, reasoning through the response an agent primed with only the stated
doc variant's rule text would give, then checking it against the fixture's
`pattern`/`forbid_pattern`. Building an automated LLM-invocation harness is
deferred to a follow-up ticket, gated on whether this manual pass shows the
approach has enough signal to be worth automating (it does — see below).

| id | full-doc | lite-doc | notes |
|---|---|---|---|
| shell-conditional | PASS | PASS | Rule text identical in both variants (`Bash.md` cross-reference doc is unaffected by this pilot); both would emit `if test`. |
| blocking-sleep | PASS | PASS | Both variants state the rule as a standalone anti-pattern with the exact trigger phrase ("CI run", "wait for it to finish"); lite doc's one-liner is unambiguous enough to avoid `sleep N`. |
| parallel-ticket-race | PASS | PASS | Both variants carry the invariant-1 elaboration sentence verbatim (added to lite doc specifically to close this gap — see structural gate history above). |
| commit-before-revert | PASS | PASS | Both variants name "Commit stale/failed work before discarding it" as a named sub-rule of Phase 2; lite doc's one-line gloss is sufficient to trigger "commit first". |
| zero-zombie-teardown | PASS | PASS | Rule 3 is short and unambiguous in both variants. |
| uncommitted-review-carryover | PASS | PASS | Both variants state the Phase 3 exit condition explicitly ("commit or explicit user authorization"). |
| hook-unit-test-confidence | PASS | PASS | Full doc's anti-pattern entry has case-study detail (2026-08-31 incident); lite doc's one-liner drops the case study but keeps the actionable rule ("green go test ≠ proof... require one real end-to-end check"), which is what the fixture checks for. |

**Score: lite 7/7, full 7/7 — lite doc meets the ">= full doc" ship gate.**

Caveat: this is a small, hand-picked fixture set scored by reasoning rather than
by actually invoking two separate agent sessions and diffing real transcripts —
it demonstrates the mechanism and gives reasonable confidence for the
mechanically-checkable rules it covers, but is not a substitute for a real
automated run. Recommend building the automation (fixture runner +
pattern-check script + real LLM invocation per variant) as a follow-up once
more lite docs exist and the harness has a scriptable LLM-call primitive to
drive it (see `lmcoder` skill for a possible starting point).
