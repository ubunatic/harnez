# 232 — `harnez issues [verb]`: single-call ticket status changes with index sync and commit-by-default

**Status**: Closed — implemented; see 232's own ticket file for design, command lives in cmd/harnez/issues.go
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: `docs/IssueTracking.md` (status schema/lifecycle invariants this must honor),
`cmd/harnez/find.go` (existing issues-file parsing/lookup; `find` stays read-only, this
command is its write-side counterpart), `cmd/harnez/index.go` /
`internal/index.UpdateIssuesReadme` (existing `harnez index` logic this would call
in-process), `internal/issues/issues.go` (`ParseIssueFile`, `CanonicalizeStatus`,
`LeadingLifecycle` — reuse for status-line parsing instead of hand-rolled regex),
`internal/usage/livefetchcache.go` + `internal/usage/history.go` (this repo's actual
concurrent-file-write idiom: non-blocking, bounded-retry `flock`, not `O_EXCL`),
[[108-subagent-dispatch-sequential-default-and-issue-number-race-guard]] (same
issues-tooling area; establishes that concurrent *agent dispatch* touching the tracker
is the real-world race scenario, not concurrent human edits),
[[126-document-closed-resolved-in-commit-self-reference-convention]] (open question this
ticket's `close` verb must not silently "solve" by faking a self-referencing commit sha)

---

## 1. Problem & Motivation

Changing a ticket's status today is four separate steps: hand-edit the `Status` line in the
ticket file (Edit tool), run `harnez index` to resync `issues/README.md`, `git add` the specific
files, then hand-compose and run a `git commit`. Every one of those is a distinct tool call for an
agent, for something that is conceptually one atomic, low-risk action performed constantly —
closing a ticket, reopening one, marking it blocked, or flipping it to in-progress.

Concretely observed this session: closing tickets 071 and 123, and splitting 224/166 into new
tickets 230/231, each required the same four-step dance by hand. A single command call would
collapse that into one. This also directly serves `docs/IssueTracking.md` §5 Lifecycle Invariant 2
("Atomic Index Synchronization") and Invariant 3 ("Immediate Tracker Commit") — those invariants
currently depend on an agent remembering and executing all four steps by hand every time;
today's tooling gives it no atomic way to comply.

## 2. Proposed Shape (refined)

```
harnez issues <verb> <ticket-number> [reason...] [flags]
```

- **Command group, not a `find` verb.** `find` is documented (`docs/CLIDesign.md`,
  `cmd/harnez/find.go`'s own doc comment) as a read-only query surface — "searches
  issues/*.md ... " with no mutation path. This is exactly the `apply`/`init` lesson from
  `docs/CLIDesign.md`: two commands that *look* like they could share a namespace but have
  different blast radii (query vs. mutate-and-commit) must stay disjoint, not be merged for
  convenience. Resolved: `issues` is a **new top-level command**, sibling to `find`/`index`,
  not a flag or subcommand bolted onto either. (`find issues history` is a precedent for
  `issues <verb>` reusing the `issues` word as a namespace, without reusing `find`'s command
  itself.)

- **Verbs are a small closed set mirroring `docs/IssueTracking.md`'s Allowed Values**, each
  taking an *optional free-text reason* appended after `— `. This is directly evidenced by
  `grep '^\*\*Status\*\*:' issues/*.md`: the canonical `Status` values (`Open`, `In Progress`,
  `Draft`) appear bare far more often than not, but every `Blocked`/`Closed` in the wild carries
  free text, and it's highly varied — `Closed — resolved`, `Closed — won't fix (...)`,
  `Closed — resolved in <commit>`, `Closed (scoped down — see ...)`, `Open — deferred, needs
  assessment before implementation`, etc. **Resolved**: the verb set stays closed and small; the
  reason argument stays free text. Do not try to enumerate resolution kinds — that reintroduces
  the hand-editing problem this ticket exists to remove, and the corpus shows no stable
  sub-taxonomy worth enforcing.

  | Verb | Resulting `Status` line |
  |---|---|
  | `open [reason]` | `Open` or `Open — <reason>` |
  | `start` | `In Progress` |
  | `block <reason>` | `Blocked — <reason>` (reason required — a bare `Blocked` with no cause is not useful to a future reader) |
  | `close [reason]` | `Closed` (bare) if no reason given — **not** an auto-fabricated `Closed — resolved`; `Closed — <reason>` otherwise. Bare `Closed` and `Closed — resolved` are both common in the corpus (18 and 15 occurrences respectively) — don't force one over the other. |
  | `draft` | `Draft` |

  Note in passing (not in scope to fix here): the corpus also has 2 occurrences of
  `Status: In Review`, which is not in `docs/IssueTracking.md`'s Allowed Values table at all.
  Worth a follow-up doc ticket to either add it as a canonical value or migrate those two
  tickets — out of scope for this command, but the command's closed verb set should *not*
  silently add an `In Review`/`review` verb to paper over that drift; flag it instead if
  `docs/IssueTracking.md` gains it later.

- **`close`'s commit-sha self-reference (see [[126]]):** do not attempt to auto-fill
  `Closed — resolved in <commit-that-doesn't-exist-yet>` — the sha is unknowable before the
  commit is created, and 126 already documents this as a known chicken-and-egg with no chosen
  resolution yet. This command should not pick option A or B on 126's behalf. Concretely: if
  `--commit` is passed to `close` with no explicit reason, write bare `Closed`, not a guessed
  sha — leave a hash-bearing reason as something the caller supplies explicitly (or corrects in
  a tiny follow-up commit, per 126's option A, which is what every ticket in the observed corpus
  already does by hand).

- **Status-line rewrite**: reuse `internal/issues.ParseIssueFile` (already locates and parses
  the `**Status**:` line) rather than a new regex. Extend it (or add a sibling
  `internal/issues.RewriteStatus(content, newStatus) (string, error)`) that replaces exactly
  that line via the same anchor `ParseIssueFile` uses for parsing, so read and write share one
  source of truth for "where the Status line is" — this directly avoids the ticket's original
  concern about disturbing surrounding content or `[[wikilink]]` text.

- After rewriting the ticket file, call `internal/index.UpdateIssuesReadme` in-process (the same
  function `cmd/harnez/index.go`'s `runIndex` already calls) — no subprocess shell-out, matching
  the ticket's original intent.

- **`--commit [message]`**: **resolved to commit-by-default**, not opt-in. This is the opposite
  of `harnez index`'s current default, and deliberately so: `docs/IssueTracking.md` §5 Invariant
  3 says tracker-metadata commits must happen *immediately*, not be deferred or batched — a
  status-change command whose default is "no git action" reintroduces exactly the multi-step
  dance this ticket exists to collapse (agent still has to remember to run `git add && git
  commit` after). Default: after a successful status change + index resync, stage exactly the
  changed ticket file and `issues/README.md` and commit, using a composed default message when
  none is given — `docs(issues): <verb> <ticket-number>[, <reason>]`, matching this repo's
  actual commit-message convention (see recent log: `docs(issues): close 228, record
  resolved-in commit sha and regenerate index`). Provide `--no-commit` as the explicit escape
  hatch for callers that want file-only edits (e.g. batching several status changes into one
  hand-composed commit) — mirrors `index --check`'s "opt out of the default action" shape rather
  than inventing a new flag idiom.

- **`--check` / `--dry-run`**: mirror `harnez index --check` (same flag name, for consistency
  across the two related commands) — report what *would* change (old status → new status,
  README diff) without writing or committing, exit 1 if there is drift from the requested state
  (e.g. `close`ing an already-closed ticket with a different reason still counts as drift; an
  identical no-op does not — see idempotency below).

## 3. Agentic-Ergonomics Requirements (new section — this is the part the original ticket
   under-specified: what makes this good for an LLM caller specifically, not just "does the
   status-change work")

- **Idempotency, precisely defined**: calling `close` on an already-`Closed` ticket must not
  error. If the resulting canonical status *and* reason text are unchanged, this is a no-op:
  exit 0, print "already closed, no change", skip the README rewrite and skip `--commit`
  (nothing to commit). If the verb would change the *reason* on an already-closed ticket (e.g.
  correcting a placeholder commit sha per [[126]] option A — a real, expected use), that is a
  genuine, non-idempotent change and should proceed normally (rewrite + index + commit). This
  distinction matters because an agent retrying a failed multi-step operation, or defensively
  re-confirming a status it already believes is set, must never receive an error for asking the
  system to be in a state it's already in — but must also not be silently blocked from a
  legitimate correction.

- **Structured output for self-verification**: this command exists so an agent doesn't have to
  re-run `find`/re-parse the ticket file to confirm its own action succeeded. Support `--json`
  (matching `find`'s `--json` convention) emitting exactly what changed:
  ```json
  {"number":"232","file":"issues/232-....md","old_status":"Open","new_status":"Closed — resolved",
   "readme_updated":true,"committed":true,"commit_sha":"abc1234","noop":false}
  ```
  Non-JSON output should be a single deterministic line, not prose, e.g.
  `232: Open -> Closed — resolved (README updated, committed abc1234)` — an agent scanning
  transcript output for confirmation should not have to parse a paragraph.

- **Fail loudly and specifically on ticket-not-found / ambiguous-number** — this is a case
  `find`'s TSV-with-empty-exit-0-on-no-match convention does *not* transfer here: a query
  returning zero rows is a valid "no results" answer, but a status-change verb given a
  nonexistent ticket number is a caller bug and must be a non-zero exit with an actionable
  stderr message (mirrors `find`'s existing "invalid entity/query" error contract, not its
  "zero matches" contract).

- **Concurrency / race safety (resolved with evidence, not speculation)**: checked
  `internal/index.UpdateIssuesReadme` (`internal/index/index.go:78`) — it does a plain
  `os.ReadFile` / `os.WriteFile` today with **no locking of any kind**. The ticket's original
  assumption that `harnez index` "presumably already has" file-locking care is **false**; it has
  none. Two genuinely different locking idioms already exist in this codebase and neither is a
  drop-in fit as-is:
  - `internal/issues.Reserve`'s `O_EXCL` retry loop (`internal/issues/issues.go:576`) is
    specifically for *allocating a new, not-yet-existing filename* — irrelevant here, since
    `issues <verb>` always targets an existing, already-numbered file.
  - `internal/usage/livefetchcache.go` and `internal/usage/history.go` establish this repo's
    actual idiom for concurrent writes to one shared file: a non-blocking, bounded-retry
    `syscall.Flock(..., LOCK_EX|LOCK_NB)`, falling back to skip-rather-than-block when the lock
    can't be acquired promptly. **This is the pattern `issues <verb>` should adopt** for the
    `issues/README.md` rewrite step specifically (the one file every invocation touches,
    regardless of which ticket number it's changing) — take a bounded-retry flock on
    `issues/README.md` (or a sibling `.lock` file next to it) around the read-modify-write in
    `UpdateIssuesReadme`, not around the whole command. The per-ticket file write needs no lock
    of its own: two `issues <verb>` calls targeting *different* ticket numbers never touch the
    same ticket file, only the shared README is genuinely contended.
  - Given [[108]]'s finding that this repo's actual practice is sequential-by-default subagent
    dispatch (not routine true concurrency), the flock is cheap insurance for the rare
    concurrent-agent case, not a load-bearing requirement — but it is a small, already-idiomatic
    addition, so there's no reason to ship without it.

- **Verb output should double as a `git commit --amend`-free correction path**: since `close`
  with a new reason on an already-closed ticket is a legitimate, expected action (see
  idempotency above, and [[126]]), the command must never refuse to re-run a verb just because
  the ticket is already in that lifecycle bucket — only exact no-ops are special-cased.

## 4. Remaining Open Questions (genuine judgment calls, not resolved here)

- **Exact verb spelling for `In Progress`**: proposed `start`, but `resume` or `wip` are
  plausible alternatives — bikeshed for whoever implements, not architecturally significant.
- **Whether `--check`'s exit-1-on-drift should apply to the reason-only-changes case** the same
  way it applies to a full status-category change, or whether that should be a softer signal
  (exit 0 with a "would update reason text" note) — needs a decision but doesn't block starting
  implementation.
- **Whether to also expose this as `harnez find issues <verb>`** as a discoverability alias
  (since agents may reasonably guess the verb lives under `find issues` by analogy to `find
  issues next`/`find issues history`) even though the underlying implementation lives in the new
  `issues` command — a thin alias is low-cost but adds a second documented spelling for the same
  action, which cuts against "predictable naming" as much as it helps discoverability. Leaning
  no (single spelling only), but worth a second opinion before implementation.
- **Archiving interaction**: `docs/IssueTracking.md` §4.2 says closed tickets move to
  `issues/archive/`. This ticket's `close` verb explicitly does **not** perform that move (per
  the original ticket's Non-Goals) — worth restating here so implementation doesn't scope-creep
  into archiving, and so `close`'s output doesn't imply the ticket has been archived when it
  hasn't.

## 5. Non-Goals (unchanged)

- Not proposing to change `docs/IssueTracking.md`'s status schema itself — this command should
  express the existing schema, not redesign it.
- Not proposing archiving (`issues/archive/`) automation as part of this ticket — that's a
  separate concern (§4.2 of `docs/IssueTracking.md`) worth its own ticket if pursued.

No implementation plan yet — filed to capture the idea; this refinement pass resolves command
namespace, verb set, default-commit behavior, status-line rewrite approach, race-safety idiom,
and idempotency semantics with concrete evidence. Remaining items in §4 are genuine judgment
calls for whoever picks this up.
