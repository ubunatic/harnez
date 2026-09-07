# 269 — harnez issues mv: renumber a ticket by number, rename file, fix header, resync README

**Status**: In Progress
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: Issue 108 (subagent dispatch sequential-default + issue-number allocation
race guard), Issue 240 (duplicate ticket numbers 179 and 180, resolved by hand),
Issue 268 (`harnez exec` --timeout, itself a real instance of this incident: locally
numbered 266, collided with a remote 266/267 at `git pull`, manually renumbered to 268),
`cmd/harnez/issues.go` (verb dispatch), `internal/issues/issues.go` (`Reserve`,
`NextNumberFromFiles`, `Scan`), `docs/practices/AgenticLoop.md` Invariant 1
(sequential dispatch as the primary mitigation for this race class)

---

## 1. Problem & Motivation

User request (near-verbatim): "add 'harnez issues mv' to rename a ticket (number only
for now, default mv target is the next free number)".

### Motivating incident (this session, 2026-09-07)

A subagent ran `harnez issues new` and filed a ticket, locally numbered 266 and
committed. Before that commit was pushed, the user separately ran `git pull`, which
brought in remote commits that had *independently* claimed ticket numbers 266 and 267
for unrelated work (an Antigravity telemetry ticket and an agy `hooks.json` fix — see
issue 268's own header, which records exactly this: "renumbered from 266 to 268").
This produced a `both modified` git conflict on `issues/README.md`'s index table.

Resolving it required, entirely by hand: `git mv issues/266-<slug>.md
issues/268-<slug>.md` (268 being the next actually-free number after resolving the
remote's 266/267), hand-editing the ticket file's own `# 266 — ...` header line to
`# 268 — ...`, resolving the README conflict by taking the remote's version, re-running
`harnez index` to regenerate the table, and then manually stripping stray unresolved
conflict-marker lines that a second overlapping commit had left in the README even
after the `index` re-run.

This exact multi-step sequence — rename file, fix in-file header number, regenerate
README, clean up any conflict artifacts — is precisely what a single `harnez issues mv`
command should automate as one atomic, tested operation, so an operator (human or
agent) does not have to reconstruct it by hand under the time pressure of a live merge
conflict.

### Why this keeps recurring

This is a structural race, not a one-off. Issue 108 already names the underlying
hazard: ticket-number allocation is a read-then-derive-then-write on shared state
(`issues/README.md` plus the `issues/*.md` file listing), and two independently
dispatched agents (or, as here, two independently-working *sessions* that only sync at
`git pull`/merge time) can both observe the same "next free number" before either
commits. Issue 108's fix — hardening `Reserve`'s `O_EXCL` claim to be number-exclusive
— closes the race *within* a single shared filesystem/working tree. It does nothing for
two clones/branches/machines that each reserve independently and only discover the
collision when they merge, which is exactly what happened here. `docs/practices/
AgenticLoop.md` Invariant 1 already says strict sequential dispatch is the primary
mitigation *within one session*; it explicitly cannot prevent a collision across
sessions that only converge at pull/merge time. This ticket is not about preventing the
race (108 already owns that) — it is about giving whoever hits the race anyway a fast,
safe, one-command recovery path, instead of the ad hoc manual sequence above.

## 2. Technical Specification / Findings

### Existing verb structure (`cmd/harnez/issues.go`)

`harnez issues <verb> <ticket-number> [reason...]` is a Cobra command (`newIssuesCmd`,
`cmd/harnez/issues.go:62`) with a closed set of status-changing verbs — `open`,
`start`, `block`, `close`, `draft` — dispatched through `composeNewStatus`
(`issues.go:166`) and executed by the shared `runIssuesVerb` (`issues.go:274`), plus a
special-cased `new` verb (`runIssuesNew`, `issues.go:206`) that takes no ticket number
and never commits. `mv` should join this verb set as a second special case (like `new`,
it doesn't fit `runIssuesVerb`'s "rewrite the Status line" shape) — add it as an
`args[0] == "mv"` branch in `RunE` alongside the existing `args[0] == "new"` branch, and
extend the `Args` validator (`issues.go:118`) so `mv` accepts 1 or 2 positional args
after the verb (old number, and an optional new number) rather than requiring the
`reason` args every other verb takes.

Ticket lookup already exists and should be reused as-is: `findTicketFile(issuesDir,
ticketArg)` (`issues.go:235`) parses the number argument, calls `issues.Scan`, and
returns the matching `issues.IssueFile` (or a "no ticket found" / "ambiguous, matches
multiple files" error) — `mv`'s old-number argument resolves through this exact
function, for the same error behavior every other verb already has.

### Number allocation logic (must match `new`/`find issues next` exactly)

Default target-number computation must reuse, not reimplement, the same logic `Reserve`
uses: `internal/issues/issues.go:592` (`Reserve`) computes the next number via
`issues.Scan(issuesDir)` -> `issues.NextNumberFromFiles(files)`
(`internal/issues/issues.go:569`), which is `maxNumberFromFiles(files) + 1` formatted
`%03d`. When `mv`'s new-number argument is omitted, call `NextNumberFromFiles` the same
way rather than re-deriving "next free" by any other means (e.g. scanning
`issues/README.md`'s table, which issue 108 already showed can be stale/wrong
independent of the file listing).

Note issue 108's open finding: today's `Reserve` claims are exclusive on the full
filename (number+slug), not the number alone, so two *concurrent* `mv` calls (or an
`mv` racing a concurrent `new`) targeting the same default "next" number could still
collide today. `mv` should claim its target file with the same `O_CREATE|O_EXCL`
pattern `Reserve` uses (never blind-`os.Rename` onto a path that might already exist)
so it fails loudly instead of silently overwriting — but closing the underlying
number-exclusivity race is issue 108's scope, not this ticket's; `mv` should build on
whatever `Reserve`/the claim mechanism looks like when this is implemented, not
duplicate or diverge from it.

### What `mv` must actually do, per file

1. Resolve `<n>` to its `IssueFile` via `findTicketFile` (existing helper).
2. Resolve the target number: `<new-n>` argument if given, else
   `issues.NextNumberFromFiles(issues.Scan(issuesDir))` (existing helper). Reject if
   `<new-n>` is already claimed by an existing file (same "no collision" check
   `Reserve` needs per issue 108) — `mv` must never silently overwrite an existing
   ticket.
3. Rename the file: `issues/<old>-<slug>.md` -> `issues/<new>-<slug>.md`, keeping the
   same slug unchanged (per the "number only for now" scope below) — via `os.Rename`
   guarded by the existence check in step 2, analogous to `git mv` (this command should
   probably shell out to `git mv` when the working tree is a git repo, so the rename is
   tracked as a rename in history rather than an add+delete — check how other file
   moves in this codebase are handled, if any, and match that convention; falling back
   to a plain `os.Rename` outside a git repo).
4. Rewrite the in-file header: the first line, `# <old> — <title>`, becomes
   `# <new> — <title>`. **No existing helper does this** — `ParseIssueFile`
   (`internal/issues/issues.go:162`) only *parses* title/status, and `Reserve`'s header
   line (`internal/issues/issues.go:615`) is only ever *written fresh*, never rewritten
   in place. This needs a new small function, e.g. `RewriteHeaderNumber(content,
   newNum string) (string, error)`, parallel in shape to the existing
   `issues.RewriteStatus` (used by `runIssuesVerb`, `issues.go:313`) — match its
   error-handling convention (e.g. what it returns when no `# NNN — ...` line is found)
   rather than inventing a new convention.
5. Regenerate `issues/README.md` via the same in-process path every other verb uses:
   `index.UpdateIssuesReadme(readmePath, issuesDir)` (see `runIssuesVerb`,
   `issues.go:328`/`351`) — do not shell out to `harnez index` as a separate process
   call; call the shared function directly, exactly like `runIssuesVerb` already does.
6. Commit (unless `--no-commit`/equivalent is passed, matching the other verbs'
   `--no-commit` flag), staging exactly the renamed file (new path; git records the old
   path as deleted automatically for a tracked rename) and `issues/README.md` — nothing
   else. Default commit message shape:
   `docs(issues): renumber <old> to <new>` (or similar; match
   `defaultCommitMessage`'s existing pattern at `issues.go:378`).

### Non-goals / explicit out of scope ("number only for now")

Per the user's explicit framing, this ticket scopes the initial implementation to
**renumbering only** — old-number to new-number, same slug, same content, same title
text apart from the number in the header line. Explicitly **not** in scope for this
ticket (do not scope-creep into these without a separate ticket):

- Renaming/changing the **slug** portion of the filename independent of the number
  (e.g. correcting a typo'd slug, or re-deriving the slug from an edited title).
- Any **content editing** — title text (beyond the number itself), Status,
  Priority/Severity/Category, or body changes.
- Fixing up **cross-references** to the old number in *other* tickets' `**Related**:`
  lines or prose (issue 240's manual resolution had to grep for these by hand; this
  ticket does not attempt to automate that grep-and-fix sweep — a future ticket could,
  but it is materially riskier than a single-file rename since it touches arbitrarily
  many other files and would need to distinguish real number references from unrelated
  numeric text).
- Batch/multi-ticket renumbering in one call (e.g. resolving an entire block of
  colliding numbers at once, as issue 240 needed for two separate pairs). One `mv`
  call moves exactly one ticket.

## 3. Implementation & Verification Plan

### Implementation sketch

- `internal/issues/issues.go`: add `RewriteHeaderNumber(content, newNum string)
  (string, error)` (regex on the first `# <num> — ` line; error if not found or
  malformed, mirroring `RewriteStatus`'s conventions). Add tests in
  `internal/issues/issues_test.go` covering: normal header, header missing entirely,
  header with unusual spacing around the em dash.
- `cmd/harnez/issues.go`: add `runIssuesMv(w io.Writer, dir, oldArg, newArg string,
  opts issuesRunOptions) (issuesResult, error)` implementing the six steps above; wire
  it into `RunE`'s verb switch and extend the `Args` validator and `--help` `Long` text
  (the verb table currently documents `open/start/block/close/draft/new` — add `mv`
  with the same style of explanation `new` gets). Reuse `issuesRunOptions` and
  `issuesResult` where they fit (e.g. `NoCommit`, `JSON`, `Check`/dry-run semantics) so
  `mv` behaves consistently with the rest of the verb family rather than inventing
  parallel flag/output plumbing.
- Collision guard: claim the destination filename with `O_CREATE|O_EXCL` (matching
  `Reserve`'s approach) before renaming, so a concurrent claim of the same target
  number fails loudly instead of overwriting.

### Verification (must include a collision/rename-scenario test, not just a happy path)

1. Unit tests for `RewriteHeaderNumber` (above).
2. `cmd/harnez/issues_test.go`: happy-path `mv <old> <new>` — file renamed, header
   updated, README regenerated with the new number/path, commit created with expected
   staged paths and no other working-tree changes swept in.
3. `mv <old>` with no `<new-n>` — asserts the target equals
   `NextNumberFromFiles(Scan(...))` computed independently in the test, i.e. behaves
   identically to what `harnez issues new`/`harnez find issues next` would report as
   the next free number at that point.
4. **Collision-scenario test simulating the actual git-pull collision this ticket is
   grounded in**: seed a temp `issues/` dir with two files claiming the same number
   under different slugs (reproducing the real 266/266 shape from this session, and the
   179/179 and 180/180 shape from issue 240) — i.e. simulate the "two sessions
   independently reserved the same number, now merged into one working tree" state —
   then run `mv` on one of them to the next free number and assert: the renamed file
   exists at the new path with the corrected header, the old numbered slot now has
   exactly one remaining file, `issues/README.md` has no duplicate-number rows and no
   leftover conflict-marker artifacts (`<<<<<<<`, `=======`, `>>>>>>>` literal strings),
   and `harnez status`'s tracker linter (`internal/issues` `LintFS`, see issue 108)
   reports zero `DiagDuplicateNumber` diagnostics against the resulting directory.
5. Attempting `mv` onto an already-occupied target number errors without modifying
   either file or the README (no partial rename left on disk).
6. `--no-commit` leaves the rename and README update on disk but stages/commits
   nothing, matching the other verbs' `--no-commit` contract.

### Risks / open questions

- Whether to shell out to `git mv` vs. `os.Rename` + `git add` needs to match whatever
  convention `gitAddAndCommit` (`issues.go:391`) already uses for staging — check it
  during implementation rather than assuming; the goal is a real git rename in history,
  not add+delete, but only where the existing commit helper already talks to git in a
  compatible way.
- This ticket depends on `Reserve`'s destination-claim shape only loosely (mv performs
  its own `O_CREATE|O_EXCL` claim on the new path). If issue 108's number-exclusive
  claim mechanism lands first, `mv` should reuse it rather than duplicating a second,
  possibly divergent claim implementation — worth a quick check when picking this up.
