# Study: `harnez-tool-observability` and the Real-Environment Verification Gap

**Date**: 2026-08-31
**Scope**: End-to-end delivery of a new feature (tool-call telemetry: `harnez rate`/`harnez exec`/`harnez stats`,
plus the `apply`-managed hook wiring that makes capture automatic) via eight sequential fresh-dev-agent tickets,
and the four real bugs that shipped despite every ticket's own unit tests passing.
**Related Issues**: [115](../../issues/115-canary-duckdb-go-embedding-for-tool-telemetry.md)–[124](../../issues/124-posttooluse-internal-tool-call-auto-capture.md)
**Related Docs**: [HookRewritePattern.md](../HookRewritePattern.md), [AgenticLoop.md](../practices/AgenticLoop.md), [Canary.md](../other/Canary.md)
**Status**: Feature shipped and live-verified; process lesson folded into `AgenticLoop.md`'s Phase 3 review gate.

---

## 1. What shipped

A pasted spec (`harnez-tool-observability`) was broken into nine tickets (115–124 — 123 deliberately unscheduled)
and executed one at a time via `/fresh-sprint`-style dispatch: a fresh dev subagent per ticket, each with a
self-contained brief, each independently reviewed before moving to the next. In order:

- **115**: canary `marcboeker/go-duckdb` — **NO-GO** (hard multi-process write lock, cgo dependency).
- **116**: `internal/telemetry` storage layer, repointed at `modernc.org/sqlite` (pure Go) per 115's finding.
- **121**: `internal/resolve` — shared `session_id`/`ticket_id` resolution.
- **117**: `harnez rate` — agent self-report command for internal-tool quality scores.
- **118**: `harnez exec` + `harnez exec hook` — automatic shell-call telemetry via a `PreToolUse` rewrite.
- **119**: fold the telemetry hook into `apply`'s existing managed-hooks merge.
- **120**: `harnez stats` — aggregate reporting.
- **122**: inject a "call `harnez rate`" instruction snippet into global agent configs via `apply`.

By the letter of each ticket's acceptance criteria and test suite, all eight closed clean. `go test ./...` was
green after every single one. And the feature did not actually work.

## 2. Four bugs, two different discovery mechanisms

| # | Bug | Found by | Would tests have caught it? |
|---|-----|----------|------------------------------|
| 1 | Two independently-shipped `PreToolUse`/`Bash` hooks (119's new one, 069's pre-existing distill one) race — Claude Code runs multiple hooks matching one event in parallel and the last `updatedInput` to finish wins, non-deterministically | Manual review, cross-referencing Claude Code's own hooks-guide documentation, *before* the code was accepted | No — each ticket's tests only exercised its own hook in isolation |
| 2 | `harnez exec hook`'s rewrite spliced the raw original command after `--`; any shell metacharacter (`|`, `&&`, `;`) in it got re-interpreted by Claude Code's own outer `bash -c` instead of reaching `harnez exec`'s argv | Manual review, reasoning through what the *outer* shell actually does with the rewritten string | No — the shipped test asserted the literal (broken) rewrite string as "correct" |
| 3 | `distilled_bytes` was `NOT NULL DEFAULT 0` in the shipped schema, contradicting the ticket's own "NULL when not distilled" acceptance criterion | Manual AC review while implementing a *different* ticket (120) that needed to query the column | No — no test inserted a row without `distilled_bytes` set |
| 4a | `internal/resolve.Ticket`'s branch-name-shaped-ticket heuristic assumed agents branch per ticket; this repo (and the user) never does, and a session may start outside any git repo — the heuristic could never fire for real usage | **Live testing**: user restarted the session, ran real Bash commands, `harnez stats` showed nothing | No — every test supplied either an explicit ticket or a fixture repo on a ticket-shaped branch |
| 4b | `Ticket()`'s hard-error-on-unresolved turned bug 4a from "unhelpful" into "actively broken": `recordExecTelemetry` treated the error as fatal and dropped the *entire* telemetry row, not just the ticket field | Same live-testing pass, once new debug logging traced the actual call path | No — same reason as 4a |
| 5 | The schema-version guard (added specifically to fix bug 3's *class* of problem) had its own bootstrapping bug: `PRAGMA user_version == 0` is ambiguous between "brand-new file" and "file that predates the guard itself" — the guard silently trusted a real pre-existing stale DB | Same live-testing pass, second bug found after fixing 4a/4b and re-testing | No — the one test for this guard used a file the guard's own code had created and then hand-edited, which doesn't reproduce "created entirely before the guard shipped" |

Bugs 1–3 were caught by *reading the diff carefully and reasoning about what the code actually does* — no code ran.
Bugs 4 and 5 were only found by *actually restarting a session and using the feature for real* — reasoning about
the code in the abstract had already happened (multiple review passes, all eight tickets) and still missed them.

## 3. Why review and tests both missed 4 and 5

**Bug 4 (ticket-resolution heuristic + hard error)**: the ticket that built this (121) was reviewed and its tests
were genuinely rigorous — explicit-override-wins, PPID-hash stability, sliding-window lock file, nested-subdirectory
repo-root detection, all covered. What none of the tests did was ask "what happens when *none* of these signals are
present, in the environment this actually runs in." Every fixture either supplied an explicit ticket or built a
throwaway git repo checked out to a ticket-shaped branch name — because that's what the *spec* assumed a realistic
caller would look like. The spec was wrong about the caller's actual workflow (confirmed directly by the user:
"I always work on a main branch. I never create ticket branches... any ticket determination logic should be quite
resilient"), and nothing in the review process cross-checked the spec's assumption against how this repo — or its
user — actually operates. A unit test built on a false premise passes and proves nothing.

**Bug 5 (schema-version guard)**: this one is more interesting because it's a bug *in a fix for bug 3*, added the
same day, with its own regression test — and the test still didn't catch it. The test created a file, let the guard
stamp it, then hand-edited `PRAGMA user_version` down to simulate staleness. That reproduces "a file the guard has
already seen once." It does not reproduce "a file that existed before the guard's code ever ran against it" —
which is exactly what the real `~/.harnez/tool_catalog.sqlite` on this machine was, having been created earlier the
same session before the guard was written. The fix's own test was shaped by the same mental model as the fix
itself, so it couldn't expose the fix's blind spot.

**The common thread**: fixtures and hard-error contracts both encode assumptions about "what a caller/environment
looks like." When those assumptions come from a spec or from the code's own prior design rather than from checking
against reality, tests built on top of them will pass while proving nothing about the actual failure mode.

## 4. How the bugs were actually found

Not by more code review — by adding an observability capability the tool *itself* was missing. `harnez` had no
log files at all until this session; every `PreToolUse` hook and `harnez exec` invocation was a black box once it
ran outside a test harness. A small `debugLog` helper (gated behind the pre-existing, previously-unused `DEBUG` env
var, appending to `~/.harnez/debug.log`) made every hook invocation, rewrite decision, and telemetry-write outcome
visible in three lines of trace per Bash call. That trace immediately showed: the hook *was* firing, the rewrite
*was* correct, the wrapper *was* running — and then a swallowed `resolve.Ticket` error, then (after fixing that) a
raw SQL constraint error, both previously invisible because `recordExecTelemetry`'s telemetry write is deliberately
best-effort/non-blocking (a correct design choice for not stalling the wrapped command — but it also means failures
were silent by construction until logging existed).

The lesson generalizes: a feature whose entire purpose is capturing what happened had, until this session, no way
to observe what it itself was doing. Building the debug-log capability wasn't scope creep on the bug hunt — it was
the prerequisite for the bug hunt to be possible at all.

## 5. What changed as a result

- `internal/resolve.Ticket` no longer infers anything from git branch names (removed entirely) and no longer
  hard-errors when nothing is resolvable — an unresolved `ticket_id` is now the expected, normal case.
- `internal/telemetry.Open`'s schema-version guard now checks table pre-existence via `sqlite_master` *before*
  trusting `PRAGMA user_version`, so it can tell "this call just created the table" from "this table already
  existed, for any reason."
- `cmd/harnez/exec.go` gained `debugLog`, a permanent, opt-in (`DEBUG=true`) tracing capability — the first log
  file this project has ever had.
- `harnez stats --auto` was added the same session (filters to the current session, resolved the same way
  `rate`/`exec` do) — a direct, small, immediately-useful outcome of having just spent an hour proving the
  resolution chain actually works.
- `AgenticLoop.md`'s Phase 3 review gate gained an explicit requirement: hook-installing or environment-resolution
  features need one live, real-environment check after an actual restart/re-apply, not just green tests — see that
  doc's "Live/Real-Environment Verification" checklist item and matching anti-pattern.

## 6. What I'd do differently

- **Run the live check as part of closing 119, not two conversation turns later.** Ticket 119 (fold the hook into
  `apply`) is precisely the ticket where "does this actually fire on a real Bash call" was the acceptance
  criterion that mattered most, and it was verified only via `harnez status`/`harnez diff` (config-drift checks),
  not by using the feature. The gap between "config says the hook is installed" and "the hook produces a row" was
  exactly where both live-found bugs lived.
- **Ask "what does this assume about the caller's real workflow" before writing the resolution-chain ticket
  (121), not after it broke.** The spec's ticket-shaped-branch assumption was never checked against this repo's
  own conventions (`docs/lang/Git.md`: "work on the default branch") even though that doc was already bundled and
  already contradicted the assumption being built.
- **Distrust a same-day fix's own test more than usual.** Bug 5 is a fix for bug 3, written and tested in the same
  session, by the same reasoning. A fix's test inherits the fix's blind spots more often than an independent
  reviewer would.
