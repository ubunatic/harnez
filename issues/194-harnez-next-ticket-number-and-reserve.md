# 194 — `harnez find issues next` (or similar) to report and reserve the next free ticket number

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: `internal/find` (`cmd/harnez/find.go`, `internal/find/search.go`,
`internal/find/query.go`), `cmd/harnez/index.go` (`issues/README.md`
regeneration), `docs/IssueTracking.md` (ticket metadata/numbering
conventions)

## Problem

Every agent (including me, repeatedly, this session) allocates the next
harnez ticket number the same manual, racy, undocumented way:
`ls issues | grep -oE '^[0-9]+' | sort -n | tail -N`. This works but:

- It's pure convention, not a command — every fresh agent has to
  rediscover/reinvent it (or worse, guess wrong and collide with another
  in-flight ticket, especially now that multiple agents/sessions — this one
  and the user's parallel session — routinely file tickets concurrently in
  the same repo, per this session's own experience with 179 vs. 186-188 vs.
  189-192 vs. 193 all landing close together).
- It has no reservation step. Two agents (or two fresh-sprint dispatches)
  computing "next number" at nearly the same time can both land on the same
  number and create colliding ticket files, especially under this project's
  now-common pattern of dispatching several fresh agents in a session.
- `CLAUDE.md`/`AGENTS.md` conventions already mandate `harnez find`/`harnez
  index` over raw `ls`/`grep`/`find` for *searching* issues — but nothing
  covers *allocating a new one*, so agents fall back to the exact raw-`ls`
  pattern those docs tell them not to use, just for this one operation.

Note: per the user's redirect earlier this session, ticket-numbering races
are considered rare in practice and not itself the priority — but the
underlying request here (making `harnez` more useful/informative *when
called by agents*, via a real command instead of ad hoc shell) is squarely
in scope regardless of how often a collision would actually occur.

## Proposal

Add a `harnez` subcommand (exact placement TBD by implementer — likely
`harnez find issues next` under the existing `find` command, or a small new
top-level command if that fits the CLI's existing structure better; check
`cmd/harnez/find.go`'s subcommand wiring before choosing) that:

1. **Reports** the next free ticket number by scanning `issues/*.md` (and
   `issues/archive/*.md` if archived tickets also consume the numbering
   sequence — confirm current convention) the same way `internal/find`
   already indexes issues, not via a fresh `ls`/regex reimplementation.
2. Supports a `--reserve` flag that additionally **claims** that number so a
   concurrent caller doesn't get the same answer — the simplest correct
   mechanism is probably writing a minimal placeholder file at
   `issues/<N>-reserved.md` (or similar) atomically (`os.OpenFile` with
   `O_CREATE|O_EXCL`) so a second concurrent call sees `N` already taken and
   returns `N+1` instead. Document whatever placeholder convention you
   choose in `docs/IssueTracking.md` so `harnez index`/`harnez find` know to
   either skip it or treat it as a real (if empty) ticket pending a real
   title.
3. Prints machine-parseable output (bare number on stdout, or `--json`) so
   an agent can capture it directly into a shell variable or ticket filename
   without scraping prose.

## Acceptance Criteria

1. `harnez find issues next` (or chosen command) prints the correct next
   free ticket number, matching what today's manual `ls | grep | sort | tail`
   convention would compute, verified against the current `issues/` state.
2. `--reserve` claims the number such that two back-to-back invocations
   (simulating near-concurrent callers) never return the same number.
3. The reservation mechanism doesn't break `harnez index`/`harnez find`'s
   existing issue-scanning — a reserved-but-not-yet-titled placeholder must
   not corrupt `issues/README.md` generation or appear as a bogus real
   ticket in search results (or, if it intentionally does appear as a
   stub entry, that's documented as the chosen behavior).
4. `docs/IssueTracking.md` updated to document the new command and the
   reservation convention, replacing the old "use `ls`/`grep`" folk process
   this ticket is fixing.
5. Existing CLAUDE.md/AGENTS.md guidance (`Issue Tracker Discovery`) gets a
   short addition pointing agents at the new command for *allocating* ticket
   numbers, alongside the existing `harnez find`/`harnez index` guidance for
   searching/indexing.
6. Unit tests cover: next-number-on-empty-dir, next-number-with-gaps
   (confirms it takes max+1, not the first gap, matching current convention
   — or documents a deliberate change if gap-filling is chosen instead),
   `--reserve` collision avoidance, and reserved-placeholder handling by
   `harnez index`.
7. `go test ./...` passes.
