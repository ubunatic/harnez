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

---

## Token-cost baseline: 1 canary-agenticloop-lite unit (issue 411, 2026-09-17)

Issue 411 asked for the session's total token use and final context size
expressed as a multiple of "1 canary-agenticloop-lite unit" — the cost of one
`hello` fixture run. Two independent pieces were needed; only one turned out
to be available.

**Piece 1 — per-fixture-run token cost, now measurable.** Canary-first probe
confirmed both CLIs expose parsed usage under `--output-format json`:
`claude -p --output-format json` returns a `usage` object (`input_tokens`,
`output_tokens`, `cache_read_input_tokens`, `cache_creation_input_tokens`,
etc.); `agy -p --output-format json` returns a flatter `usage` object
(`input_tokens`, `output_tokens`, `thinking_tokens`, `cache_read_tokens`,
`total_tokens`). Added `canary-agenticloop-lite measure-cost`, which runs the
`hello` fixture once per agent (against the full `docs/AgenticLoop.md` variant)
using the JSON output mode and prints the parsed usage. Measured baseline:

| agent | 1 unit (total tokens) | input | output | cache_read | cache_creation | thinking |
|---|---|---|---|---|---|---|
| claude | 60278 | 4 | 83 | 38093 | 22098 | 0 |
| agy | 40685 | 40351 | 334 | 0 | 0 | 199 |

Caveat: per the `agy` cwd-isolation limitation documented above, `agy -p`
does not actually read the harness's isolated per-fixture `AGENTS.md` copy —
it reads the real `~/AGENTS.md`. The agy baseline above is therefore a real
measured cost, but of agy's actual global context, not of the `hello` fixture
against `docs/AgenticLoop.md` specifically; treat the `claude` baseline as the
more meaningful of the two until agy's isolation gap is fixed.

**Doc-context trace — which docs are pulled into context, and their token cost.**
`measure-cost` now also traces the copied `AGENTS.md` for eager `@path.md`
include directives (recursively, resolved relative to repo root) and reports
each pulled-in file's byte size and a rough token estimate (`bytes / 4` — not
a real tokenizer count, just enough to compare doc weight at a glance):

```
claude   docs pulled into context:
           docs/AgenticLoop.md                       29277 bytes  ~ 7319 tokens
           (total)                                                ~ 7319 tokens
claude   1 unit = 60278 tokens (input=4 output=83 cache_read=38093 cache_creation=22098 thinking=0)
```

For the `hello` fixture's `docs/AgenticLoop.md` variant, only one file is
pulled in — the doc has no `@docs/...` includes of its own (its own text
explicitly warns against eager includes in global instruction templates,
Invariant 1 note). The ~7.3K token doc estimate is well under the real
60278-token `claude -p` total, confirming most of that total is Claude Code's
own baseline session overhead (tool schemas, system prompt, etc.), not the
doc content itself. Docs with real `@`-include chains (e.g. this repo's own
top-level `CLAUDE.md`, which pulls in `AGENTS.local.md`) will show multiple
rows and a larger total — worth a follow-up run once a fixture targets one of
those files directly.

**Piece 2 — this session's own live token/context usage: not available.**
No tool exposed in this Claude Code session reports live cumulative token
usage or current context size on request; there is no `claude` slash command,
CLI flag, or tool call surfaced here for a running interactive session to
introspect its own consumption mid-session. Only a completed `-p` invocation's
`--output-format json` (as used above) exposes usage, and that is a per-call
number for a *separate, non-interactive* subprocess invocation, not a running
session's live total. This is recorded as an explicit known limitation per the
ticket's own instructions, rather than silently dropped: "N canary-agenticloop-lite
units" can be computed for any *completed* `-p` call, but this host session
cannot currently quote its own live total against that baseline.

---

## Doc-compliance smoke check — `hello` fixture, both agents (2026-09-17)

Cheapest real-invocation check available: the `hello` fixture is a single
low-cost prompt/response pair used to confirm an agent actually reads its
`AGENTS.md` and follows the instruction inside it, without paying for the
full 7-fixture x 2-variant x 2-agent cross product.

`canary-agenticloop-lite run --fixture hello`:

| agent | full | lite |
|---|---|---|
| claude | PASS | PASS |

(`agy` excluded from this table per the standing cwd-isolation caveat above —
it never reads the isolated per-fixture `AGENTS.md`, so a doc-variant PASS/FAIL
for `agy` here would not be a valid signal.)

`canary-agenticloop-lite measure-cost` (real per-agent unit cost + doc-context
trace, `docs/AgenticLoop.md` variant):

| agent | 1 unit (total tokens) | docs pulled into context | doc tokens (est.) |
|---|---|---|---|
| claude | 60272 | `docs/AgenticLoop.md` | ~7319 |
| agy | 42800 | `docs/AgenticLoop.md`* | ~7319 |

*agy's number is subject to the same cwd-isolation caveat — real cost of
answering from its actual global `~/AGENTS.md`, not this repo's
`docs/AgenticLoop.md`, even though the doc-context tracer (which walks
whatever file the harness *intended* to copy in) reports the same path/size
for both rows.

Full session usage runs well above the raw doc token estimate for both agents
(60272 vs ~7319 for claude, 42800 vs ~7319 for agy) — most of the 1-unit cost
is baseline session/tool overhead, not the doc content itself.

---

## Guide: running the suite/units yourself

Binary: `canary-agenticloop-lite` (installed via `make install-tools` from this
repo root; source in this directory).

- **List available fixtures** (no agent invocation, free):
  `canary-agenticloop-lite fixtures list`
- **Show one fixture's full definition**:
  `canary-agenticloop-lite fixtures show <id>`
- **Cheap smoke check** — run just `hello` (recommended default; do not run
  the full unfiltered suite casually, it fans out to every fixture x every
  agent x both doc variants and is expensive):
  `canary-agenticloop-lite run --fixture hello`
- **Scope further** with repeatable flags, e.g. one agent only:
  `canary-agenticloop-lite run --fixture hello --agent claude`
  or one doc variant only: `--variant full`
- **Run a specific behavioral fixture on demand** once `hello` looks healthy,
  e.g.: `canary-agenticloop-lite run --fixture blocking-sleep --agent claude`
- **1-unit token cost baseline** (real `-p --output-format json` usage,
  now including the doc-context trace — which files get pulled into context
  and their estimated token size):
  `canary-agenticloop-lite measure-cost` (or `--agent claude` / `--agent agy`
  to scope to one CLI)
- **Override the fixtures file** (e.g. testing a fork/variant) with the global
  `--fixtures-file <path>` flag on any subcommand.
- **Compare doc-delivery modes** with `--link soft|hard|embed` (default
  `soft`) on `run` or `measure-cost`, e.g.:
  `canary-agenticloop-lite measure-cost --fixture hello --link embed`

Cost note: every `run`/`measure-cost` invocation spawns a real non-interactive
`claude -p` and/or `agy -p` subprocess per (fixture x variant x agent) —
treat this as a metered external call, not a free local check.

---

## Output rework + first-turn baseline (2026-09-17, user follow-up)

Two follow-ups from a live pairing session after the doc-context trace above:

**1. Cleaner `measure-cost` output.** Generalized `measure-cost` to accept
`--fixture`/`--variant` (previously hardcoded to `hello`/full), print the real
response text, and reformat the report as a bordered header (fixture/variant/
prompt) plus a per-agent block: context-docs table, PASS/FAIL score against
the fixture's `pattern`/`forbid_pattern`, response text, then a token-use
table.

**2. First-turn baseline instead of guessing.** The doc-context token estimate
(~7,319 for `docs/AgenticLoop.md`) didn't explain the real gap to the total
session cost (~60K) — user asked "where does all the context come from."
Considered asking the agent to reply "START" immediately as an instrumentation
trick, but `claude -p --output-format json`'s `usage.iterations` array already
exposes a real per-turn breakdown for free, so that trick was unnecessary.
`measure-cost` now reports `first-turn` (iterations[0]'s token cost — before
any tool-call round trip) alongside `total` and `turns`:

```
token use
    first-turn          36265
    total               60272
    turns                   2
```

Root cause of the 60K vs 7.3K gap: `hello` costs 2 turns, not 1 — claude
issues a real `Read` tool call to open `AGENTS.md` rather than getting it
inlined, so the fixed session overhead (system prompt + full tool schema
definitions, independent of doc content, measured at 23,872 tokens for an
empty directory with no `AGENTS.md` at all) gets billed twice: once for the
tool-call turn, once for the final-answer turn. The doc's own ~7.3K tokens is
a real but comparatively small piece of the total; most of the cost is
Claude Code's own multi-turn bootstrapping, not the doc.

`agy`'s JSON output has a top-level `num_turns` but no per-turn breakdown, so
its `first-turn` value falls back to the run's total (`firstTurnTokens()` in
`main.go`) — a known asymmetry between the two agents' introspection, not a
bug in this harness.

---

## --link flag: soft/hard/embed doc-delivery comparison (2026-09-17, issue 412 follow-up)

Added `--link` (soft|hard|embed, default soft) to both `run` and `measure-cost`,
controlling how the doc variant is exposed inside the isolated workspace's
`AGENTS.md`, per issue 412:

- **soft**: `AGENTS.md` holds a bare-prose citation ("See AgenticLoop.md for
  your instructions/context for this session."); the doc is copied in
  alongside it but nothing forces the agent to open it.
- **hard**: `AGENTS.md` holds a single eager-include line, `@AgenticLoop.md`
  — this repo's own `harnez/CLAUDE.md` convention (`@AGENTS.local.md`).
- **embed**: the doc's full text is inlined directly into `AGENTS.md` under
  a `# AgenticLoop.md` heading (text docs only).

Real `hello`-fixture, claude/full results for each mode:

| link | turns | total tokens | first-turn |
|---|---|---|---|
| soft | 3 | 73227 | 31837 |
| hard | 3 | 73831 | 32439 |
| embed | 2 | 51696 | 31685 |

All three PASS (response "ready" in each case).

**Finding**: `hard` (`@AgenticLoop.md`) did **not** behave as an eager include
in `AGENTS.md` — it cost the same 3 turns as `soft`, meaning claude still
issued a separate `Read` tool call rather than auto-inlining the referenced
file. This contradicts the assumption written in `docs/AgenticLoop.md`'s own
Invariant-1 note ("Claude Code treats `@path` in `CLAUDE.md` as an eager
macro-include... inlining full doc files into every session") if read as
applying to `AGENTS.md` generally — the eager-include behavior that doc
describes appears to be specific to `CLAUDE.md`, not `AGENTS.md`. `embed` is
the only mode of the three that actually avoids the extra tool-call turn.

Traced doc-context accounting was generalized to walk from the workspace's
real `AGENTS.md` entry point (not the raw doc file), resolving `@path`
includes relative to the referencing file's own directory — matching this
repo's real convention (sibling references, not always repo-root-relative)
and correctly reflecting what's actually reachable from each link mode.
