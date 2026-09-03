<!-- harnez:topic: Cross-project feedback promotion (issues 202/203) and the 4-level privacy-scrubbed usage/telemetry export pipeline (issue 204), plus a second case of independent parallel-session convergence -->
# Cross-Project Feedback Promotion, Privacy-Level Exports, and Parallel-Session Convergence (Round 2)

**Scope**: `internal/feedback/feedback.go`, `cmd/harnez/find.go`, `internal/privacy/`,
`internal/telemetry/export.go`, `internal/usage/export.go`, `cmd/harnez/usageexport.go`;
issues 202, 203, 204, 208.

## 1. What happened

A `harnez feedback issue` entry describing a real harnez bug got logged from inside a
*different* project's session (`smarthome`), because that's where the agent happened to be
working when it hit the bug. `harnez feedback promote <id> -d harnez` then failed —
`promote` only ever searched the log scoped to `-d`, so a bug about harnez, reported from
outside harnez, was unreachable from harnez's own tracker.

The user's directive was explicit and became a standing rule, not a one-off fix: **feedback
about harnez must live in harnez's own `issues/`, never orphaned in another project's log.**
This produced two tickets, not one — the underlying bug (202: reserved placeholder filename
can diverge from a hand-derived slug) and the meta-bug that made it hard to even get 202 out
of `smarthome`'s log in the first place (203: `promote` can't reach cross-project entries).

## 2. Fixes

- **202** — `harnez find issues next --reserve` now prints the exact reserved path
  (`<NUMBER>\t<PATH>`) instead of just the number, so callers write ticket content to the
  path the reservation actually picked rather than re-deriving a slug from the title by
  hand and risking divergence. `docs/practices/IssueTracking.md` updated to say so.
- **203** — `internal/feedback.FindByID` globs every `*.jsonl` log under the feedback dir,
  not just the caller's own. `feedback promote` falls back to it on a local-log miss, and
  writes the "promoted" status back to the entry's *actual* origin log, not the caller's.

**Lesson**: a workflow-friction report is sometimes two tickets in disguise — the concrete
bug, and the process gap that made the bug hard to report/fix in the first place. Splitting
them (202 vs 203) kept each fix small and independently testable instead of one ticket
conflating "the filename divergence" with "the tracker can't find entries filed elsewhere."

## 3. Issue 204: privacy-scrubbed usage/telemetry export

`harnez usage export` needed to produce a file safe to publish to an external static-site
datavis consumer (`ubunatic.com`). The ticket's own spec **grew twice mid-implementation**
by a parallel session (see §4) from "one fixed scrubbing behavior" to a 4-level scheme.
Mid-build spec drift like this was surfaced to the user via `AskUserQuestion` rather than
silently absorbed or silently ignored — the user chose "commit the already-built v1, then
extend toward the new spec," which is what happened.

Final shape, in `internal/privacy` (deliberately import-free of `internal/telemetry` and
`internal/usage`, so both can depend on it with no cycle):

- **`LevelPublic`** (default) — drop all free text, normalize paths/hosts/accounts.
- **`LevelAgentSanitized`** — rewrite notes through a `NoteSanitizer` interface before
  export.
- **`LevelInternal`** — keep fields, regex-scrub in place (`ScrubText`: home paths, emails,
  token-shaped strings).
- **`LevelRaw`** — unscrubbed passthrough, local use only.

`NoteSanitizer` is a real interface, not a stub, specifically so the LLM backend is
swappable later without touching the caching/batching orchestration. The shipped backend
(`ClaudeCLISanitizer`) shells out to `claude -p --output-format json`, prompt on **stdin**
(never argv — avoids argv-length limits and a note starting with `-` being parsed as a
flag), notes JSON-marshaled into the prompt so content can't break out of its slot
regardless of quotes/newlines it contains.

**Batching was a hard requirement, not an optimization**: the user interrupted twice to
insist on it ("make sure we do llm sweep in as few LLM calls as possible" / "or use batch
calls"). `DefaultSanitizeBatchSize = 50` — cache-miss notes are deduped and chunked, never
sent one-call-per-note. Combined with a content-hash sanitization cache
(`note_sanitization_cache`), a typical export run that only has a handful of new/changed
notes since the last run finishes in a single LLM call.

**Cost/test-safety boundary**: real, billed `claude` CLI calls are fine when explicitly
user-triggered, but must never run during the default test suite. Enforced with a Go build
tag (`//go:build integration`) on the one test that actually shells out to `claude`; it also
skips itself if `claude` isn't on `PATH`. `go test ./...` never invokes it.

## 4. Second instance of independent parallel-session convergence

This is the same phenomenon documented in
[2026-09-02-git-history-telemetry-release-generics-and-multi-agent-ergonomics.md](2026-09-02-git-history-telemetry-release-generics-and-multi-agent-ergonomics.md)
recurring: a *different* Claude Code session (same git author, different `Claude-Session`
trailer) independently built nearly the entire 4-level privacy scheme (commit `035395d`)
while a dev subagent spawned in *this* session was building the same thing from the same
ticket spec, unaware of each other.

Handling, again: investigate via `git log` / `git show --stat` / `git diff` before assuming
conflict, duplication, or fabrication — never assume a subagent hallucinated an unfamiliar
commit. In this case there was no real file conflict; the two implementations converged
almost exactly. The only genuinely new material from this session's subagent was additive
test/doc coverage (`claude_sanitizer_integration_test.go`, `privacy_export_test.go` in both
`telemetry` and `usage`, a `Level 2` addendum on the ticket) — that's what got committed
(`219c212`), on top of `035395d` rather than replacing or reverting it.

**Working rule** (see [[feedback_check_session_trailer_before_flagging_fabrication]]):
concurrent Claude sessions on the same repo are a known, recurring condition here, not an
edge case — always check provenance via the commit trailer before reacting to unfamiliar
state.

## 5. Deliberate scope-splitting on close

204 shipped with JSON export only; its own spec also asked for a SQLite output format. Per
explicit instruction ("close out 204, file SQLite export as follow-up"), 204 was closed with
a resolution note and the SQLite requirement was split into a fresh, appropriately-scoped
ticket (208, P3/Low) rather than left dangling inside a closed ticket or silently dropped.
208's spec is written to reuse 204's already-scrubbed export records (`ExportToolCall`/
`ExportPoint`) rather than re-deriving privacy logic in the output-format layer — the
privacy contract stays owned by 204's builders.

**Lesson**: closing a ticket whose original scope included a feature that didn't ship is not
the same as scope creep or an unfinished ticket — explicitly forking off the deferred piece
as its own prioritized ticket keeps the tracker accurate (204 really is done; the SQLite
work really is separate, lower-priority, and not blocking anything).
