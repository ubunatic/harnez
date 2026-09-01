# 148 — `harnez index`: auto-update issues/README.md, docs/README.md, docs/studies index

**Status**: Closed — resolved in `pending` (self-ref hash fixed up in a follow-up commit, see [[126]])
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Tooling
**Related**: [[126-document-closed-resolved-in-commit-self-reference-convention]], `docs/practices/IssueTracking.md`, `docs/README.md`, `issues/README.md`

## Problem

`issues/README.md` and `docs/README.md`'s studies table are hand-maintained:
every new/closed ticket needs a manual row add/status-flip in
`issues/README.md`, and every new `docs/studies/*.md` needs a manual row add
to `docs/README.md`. Both have drifted in practice — most recently, three
studies from the 2026-08-31 instruction-distribution audit were written and
correctly linked from their tickets but never indexed in `docs/README.md`
(caught and fixed by hand during an `/evergreen` pass, commit `a988e33`).
This is exactly the kind of mechanical sync step that's easy to forget mid-
sprint and only surfaces on a later audit.

Issue metadata is regular enough to parse without real YAML frontmatter:
every `issues/*.md` file opens with a title line (`# NNN — <title>`)
followed by a bold-label block (`**Status**:`, `**Priority**:`,
`**Severity**:`, `**Category**:`, `**Related**:`) per
`docs/practices/IssueTracking.md`'s canonical header format — not a `---`
YAML frontmatter block (checked: no `issues/*.md` file uses real
frontmatter, despite that being the initial assumption for this ticket).
The command described below should parse that bold-label block directly
rather than assuming YAML.

## Prep Task — frontmatter investigation (do before Scope below)

**Conclusion (recorded 2026-09-01): keep the bold-label block, do not move to YAML
frontmatter.**

Reasoning:
- `internal/issues/issues.go` already has a battle-tested bold-label parser
  (`ParseIssueFile`, `statusLineRegex`) backing `harnez status`'s tracker linter.
  A run of `harnez status` against this repo's 143 currently-indexed tickets found
  zero status-parsing failures — the format isn't drifting or breaking in practice.
- YAML frontmatter's claimed resilience gain is diluted to near-zero here: other
  harnez-managed repos' `issues/*.md` won't be migrated in lockstep (per the ticket's
  own framing), so the parser would have to keep reading bold-label-only tickets
  forever regardless of what this repo does — meaning "switch to YAML" only adds a
  second format to support, a migration command, and dual-parse tests, without ever
  letting the bold-label path be dropped.
- `docs/studies/*.md` already shows what happens when two metadata conventions
  coexist in one repo: 2 of ~24 study files picked up `---`-delimited YAML frontmatter
  (`title`/`weight`, for the unrelated docs-copy ordering pipeline) while the other 22
  use a `**Date**:`/`**Scope**:` bold-label-ish header, and a few have neither. That's
  the drift risk (hand-edits landing in whichever format the last editor used) playing
  out for real, on a much smaller surface than `issues/*.md` would be.
- Concrete YAML-specific failure modes are real and worse than the bold-label
  equivalents: a missing/malformed closing `---` corrupts parsing of the *whole* file,
  not just one field; a merge conflict landing inside a frontmatter block breaks its
  structure outright, whereas a bad bold-label line just fails one regex match and
  leaves the rest of the block/body readable.
- This is a P3/Low, solo-repo ticket — the proportional call is to reuse the existing,
  working, already-tested parser rather than add a dual-format parser + migration
  command for a resilience gain that mostly doesn't materialize (see point 2).

Proceeding straight to the Scope below, parsing the bold-label block only (as the
ticket originally scoped before this investigation was added). No `docs/studies/`
entry filed — the investigation didn't turn up anything substantial enough to warrant
one beyond this note.

Before building the parser, investigate whether issue metadata should move
from the bold-label block to real YAML frontmatter (`---`-delimited), since
that choice determines what the parser in Scope actually needs to read:

- Investigate whether real YAML frontmatter would make `harnez index` (and
  the docs/issues index generally) meaningfully more resilient than parsing
  the bold-label block, or whether YAML frontmatter is itself likely to
  drift/break often in practice (e.g. hand-edits producing invalid YAML,
  merge conflicts inside a frontmatter block, agents forgetting closing
  `---`). Write the conclusion down (a short note in this ticket or a
  `docs/studies/` entry if it's substantial) before proceeding.
- **If the conclusion is "yes, use YAML"**:
  - The parser must read *both* formats — real YAML frontmatter and the
    existing bold-label block — since old tickets in this repo and tickets
    in other harnez-managed repos will still be bold-label-only for a long
    time (no forced mass-migration).
  - Add a migration command (e.g. `harnez migrate-frontmatter` or a mode of
    `harnez index`) that rewrites a bold-label ticket to YAML frontmatter,
    non-destructively and idempotently.
  - Consider a hybrid split: YAML frontmatter carries only the
    index-relevant fields (Status/Priority/Severity/Category/Related — the
    fields `harnez index` actually needs), while richer prose-adjacent
    metadata stays in the body. Whatever the split, **the parser must keep
    reading bold-label-only tickets correctly** — this is a hard backward-
    compatibility requirement, not a nice-to-have, since other
    harnez-managed repos' `issues/*.md` files won't be migrated in lockstep
    with this repo.
- **Once a conclusion and the resulting command(s) are settled**, update
  `docs/practices/IssueTracking.md` (the canonical metadata-header spec)
  and this repo's own `AGENTS.md`/`CLAUDE.md` references to match — don't
  ship a format change or new command without updating the doc that defines
  the convention.

## Scope

- Add a `harnez index` (or similar) subcommand that:
  - Scans `issues/*.md` and `issues/archive/*.md`, parses the title +
    bold-label metadata block from each, and regenerates `issues/README.md`'s
    table rows (number, filename link, title, status — matching the current
    hand-written format).
  - Scans `docs/studies/*.md` (and optionally `docs/lang|practices|other/*.md`),
    extracts title + a topic/summary line, and regenerates the corresponding
    `docs/README.md` table(s).
  - Is idempotent and diff-clean on repeated runs against unchanged source
    files, matching the `apply`/`diff` idempotency convention already used
    elsewhere in this codebase.
- Decide and document where the "topic" summary text for docs/studies rows
  comes from, since studies don't have a structured metadata block the way
  issues do (candidates: a required `**Summary**:` line under the date/scope
  header, or the first paragraph, or a manual-override table for rows that
  need curated phrasing).
- Wire into the fresh-sprint teardown step (`docs/practices/AgenticLoop.md`
  Phase 5 / `fresh-sprint` skill step 5) as a suggested check, similar to how
  `harnez status` is already called out there.
- Out of scope: reformatting or restructuring the existing hand-written
  tables; this command should regenerate the same shape, not redesign it.

## Acceptance Criteria

- [x] Frontmatter investigation done, conclusion recorded (YAML vs.
      bold-label-only), before Scope work starts. Conclusion: keep
      bold-label, see the Prep Task section above.
- [x] If YAML was chosen: parser reads both bold-label and YAML tickets;
      a migration command exists; backward compatibility with unmigrated
      tickets (this repo and others) is verified, not just assumed.
      N/A — YAML was not chosen.
- [x] `docs/practices/IssueTracking.md` (and any other doc defining the
      metadata-header convention) updated to match whatever was decided and
      shipped. Also updated `docs/README.md` (docs/studies topic-derivation
      note), `docs/practices/AgenticLoop.md` and `commands/fresh-sprint.md`
      (Phase 5 / step 5 teardown now calls out `harnez index`).
- [x] `harnez index` regenerates `issues/README.md` from `issues/*.md` +
      `issues/archive/*.md` metadata. Run against this repo's real ticket
      set: titles/statuses are now read verbatim from each ticket's H1 and
      `**Status**` field rather than the old hand-normalized phrasing, which
      is a real (and desired) diff correcting drift the hand table had
      already accumulated — not a generator bug. See the commit that ships
      this for the full before/after.
- [x] `harnez index` regenerates `docs/README.md`'s studies table from
      `docs/studies/*.md`. Run against this repo's real studies directory:
      it found and added 3 studies that were missing from the hand-written
      table (`2026-08-26-cross-repo-managed-docs-and-agentic-tooling-design.md`,
      `2026-08-28-fresh-sprint-rograph-three-ticket-token-study.md`,
      `RTKShellWrapperHandling.md`) — the exact kind of drift this ticket
      exists to prevent. 4 studies whose auto-derived topic came out too
      long/generic got a `<!-- harnez:topic: ... -->` override comment added
      to restore their curated one-liner (see `docs/README.md`'s new note on
      the derivation/override convention).
- [x] Command is idempotent (`harnez index` twice produces no further diff;
      `harnez index --check` exits 0 after a run, 1 before).
- [x] `go test ./...` passes (including new `internal/index` tests covering
      table rendering, idempotency, and the topic-derivation priority order).

## Notes

Keep scope tight to `issues/README.md` + `docs/studies/` first; the
`docs/lang|practices|other/*.md` tables carry hand-curated "Read
Trigger/Scope" phrasing that's harder to auto-derive and can be a follow-up
if the studies/issues pass proves out well.
