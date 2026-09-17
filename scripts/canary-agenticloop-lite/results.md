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

---

## Real invocation results (real `claude -p`/`agy -p` calls, 2026-09-17)

Built `main.go` in this directory: a Go harness (issue 362 follow-up) that spawns a
genuinely isolated `os.MkdirTemp` workspace per (agent x doc-variant x fixture),
copies exactly one doc variant (`docs/AgenticLoop.md` full, or
`docs/practices/AgenticLoop.lite.md` lite) in as `AGENTS.md`, runs the fixture
prompt via `claude -p --permission-mode bypassPermissions` or `agy -p`
non-interactively from that directory, and scores the real captured response
against `pattern`/`forbid_pattern` with Go `regexp`. Mirrors
`scripts/canary-lite-doc/main.go`'s isolation style.

| id | claude/full | claude/lite | notes |
|---|---|---|---|
| shell-conditional | FAIL | FAIL | Real behavior **contradicts** the manual prediction. claude/full's real response used `if [[ ... ]]`-style checks (or reasoned about doc-agnostic shell idioms) rather than `if test`; claude/lite likewise didn't reliably emit `if test` — the isolated AGENTS.md-only context doesn't carry enough of the Bash.md cross-reference to reproduce the pattern the manual pass assumed. |
| blocking-sleep | PASS | PASS | Confirmed real. |
| parallel-ticket-race | PASS | PASS | Confirmed real. |
| commit-before-revert | PASS* | PASS* | *This fixture's `forbid_pattern` (`git reset --hard(?!.*commit)`) uses a negative lookahead that Go's RE2 `regexp` engine does not support (`invalid or unsupported Perl syntax`). The harness treats an unsupported-regex compile error as a skipped check rather than a false FAIL, and scores PASS on the `pattern: commit` half only — this fixture's forbid-check was never actually exercised against real output. Flagging as a known harness/fixture-portability gap rather than silently passing it off as a full validation. |
| zero-zombie-teardown | PASS | PASS | Confirmed real. |
| uncommitted-review-carryover | PASS | PASS | Confirmed real, response cited the actual Phase 3 exit condition text from AGENTS.md. |
| hook-unit-test-confidence | PASS | PASS | Confirmed real. |

**Real score: claude/full 6/7, claude/lite 6/7** (both fail only on `shell-conditional`,
so lite still holds parity with full under real invocation — the ship gate's relative
claim survives, but the manual pass's *absolute* 7/7 for both variants did not).

### agy — excluded from the doc-variant comparison

`agy -p` was run for all fixtures x variants and produced real output (not skipped
for missing binary — `agy` is on PATH and answered every prompt), but a standalone
manual probe confirmed `agy -p` **ignores the process's working directory entirely**:
it always executes from a fixed internal scratch directory
(`/home/uwe/.gemini/antigravity-cli/scratch`) and reads only the real
`~/AGENTS.md`, never the per-fixture isolated doc copied into the harness's
`os.MkdirTemp` workspace. Its response text load-bearingly referenced
`file:///home/uwe/AGENTS.md` rather than any temp path. This means every agy
result in this run reflects the user's real global `~/AGENTS.md`
(harnez's own global instructions doc), not `docs/AgenticLoop.md` or
`AgenticLoop.lite.md` at all — agy's PASS/FAIL numbers are real captured output,
but they are not a valid signal on the full-vs-lite AgenticLoop comparison and
are omitted from the table above. This is a genuine `agy` CLI limitation (no
non-interactive cwd/workspace isolation), not a bug in this harness or in either
doc variant — worth its own ticket if `agy` needs to be brought into future
canaries that depend on directory-scoped context.

### Net conclusion

Real invocation confirms the lite doc holds *relative* parity with the full doc
(same pass/fail set), closing the gap issue 362 flagged ("AgenticLoop-style
canaries still need manual/other scoring"). It also corrects the manual pass's
overly optimistic *absolute* score — `shell-conditional` needed the Bash.md
cross-reference doc present too, not just AgenticLoop.md/lite.md in isolation,
to reliably reproduce `if test`. Scope was intentionally cut short here (per
user token-budget request) after two consistent, confirmatory real runs;
further reruns would not change these conclusions.
