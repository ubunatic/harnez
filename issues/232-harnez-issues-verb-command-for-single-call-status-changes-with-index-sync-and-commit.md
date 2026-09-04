# 232 — `harnez issues [verb]`: single-call ticket status changes with index sync and optional `--commit`

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: `docs/IssueTracking.md` (status schema/lifecycle invariants this must honor),
`cmd/harnez/find.go` (existing issues-file parsing/lookup), `cmd/harnez/index.go` (existing
`harnez index` this would call internally), [[108-subagent-dispatch-sequential-default-and-issue-number-race-guard]]
(same issues-tooling area, same file-write-race concerns)

---

## 1. Problem & Motivation

Changing a ticket's status today is four separate steps: hand-edit the `Status` line in the
ticket file (Edit tool), run `harnez index` to resync `issues/README.md`, `git add` the specific
files, then hand-compose and run a `git commit`. Every one of those is a distinct tool call for an
agent, for something that is conceptually one atomic, low-risk action performed constantly —
closing a ticket, reopening one, marking it blocked, or flipping it to in-progress.

Concretely observed this session: closing tickets 071 and 123, and splitting 224/166 into new
tickets 230/231, each required the same four-step dance by hand. A single command call would
collapse that into one.

## 2. Proposed Shape

`harnez issues <verb> <ticket-number> [reason/message] [flags]`

- **Verbs** map to `Status` header values per `docs/IssueTracking.md`'s schema:
  - `open` → `Open`
  - `close <reason>` → `Closed — <reason>` (reason may itself reference a commit sha, per this
    tracker's `Closed — resolved in <commit>` convention — see [[126]] for how that
    self-reference is meant to work)
  - `block <reason>` → `Blocked — <reason>`
  - `start` (or similar) → `In Progress`
  - Exact verb set / naming is open for design — should stay a small, closed set mirroring the
    **Allowed Values** table in `docs/IssueTracking.md`, not a free-form string.
- After rewriting the ticket's `Status` line, run the equivalent of `harnez index` in-process
  (not a subprocess shell-out) to resync `issues/README.md` in the same command invocation.
- **`--commit "<message>"`** (optional): stage exactly the changed ticket file plus
  `issues/README.md` and create a git commit with the given message, respecting this repo's
  existing attribution/commit-message conventions. Without the flag, the command only edits
  files — no git action — matching how `harnez index` behaves today.

## 3. Open Questions / Design Risks

- **Command namespace**: no `issues` top-level command exists yet (`cmd/harnez/find.go`,
  `cmd/harnez/index.go` are separate commands). Decide whether this is a new `harnez issues`
  command group, or verbs added to an existing command — the `apply`/`init` scope-separation
  precedent in `docs/CLIDesign.md` argues for **not** overloading an unrelated command.
- **Status-line parsing/rewrite robustness**: must locate and replace exactly the `**Status**:`
  line without disturbing surrounding content or `[[wikilink]]` references inside the reason text
  — this is the same file-editing risk class already flagged in [[108]] for issue-number
  allocation.
- **`--commit` composing a message vs. accepting one**: should it always require an explicit
  message, or offer a sane default (`docs(issues): <verb> <ticket-number>`) when omitted?
- **Race safety**: if this becomes a common agent action, concurrent `harnez issues close`
  invocations across two agents touching different tickets must not corrupt
  `issues/README.md` — needs the same file-locking care `harnez index` presumably already has
  (verify, don't assume) plus whatever `find issues next --reserve`'s `O_EXCL` mechanism already
  established as this project's locking idiom.
- **Verb set completeness**: does it need to support the free-text status variants seen in
  practice (e.g. `Open — architecture decided, implementation split into ...`, `Open — deferred,
  needs assessment before implementation`) or only the canonical Allowed Values? A too-rigid verb
  set may not cover real usage; a too-free one reintroduces the hand-editing problem this ticket
  exists to remove.

## 4. Non-Goals (For Now)

- Not proposing to change `docs/IssueTracking.md`'s status schema itself — this command should
  express the existing schema, not redesign it.
- Not proposing archiving (`issues/archive/`) automation as part of this ticket — that's a
  separate concern (§4.2 of `docs/IssueTracking.md`) worth its own ticket if pursued.

No implementation plan yet — filed to capture the idea; needs a design pass (command shape,
verb set, locking strategy) before implementation starts.
