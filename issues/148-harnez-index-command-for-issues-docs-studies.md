# 148 — `harnez index`: auto-update issues/README.md, docs/README.md, docs/studies index

**Status**: Open
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

- [ ] Frontmatter investigation done, conclusion recorded (YAML vs.
      bold-label-only), before Scope work starts.
- [ ] If YAML was chosen: parser reads both bold-label and YAML tickets;
      a migration command exists; backward compatibility with unmigrated
      tickets (this repo and others) is verified, not just assumed.
- [ ] `docs/practices/IssueTracking.md` (and any other doc defining the
      metadata-header convention) updated to match whatever was decided and
      shipped.
- [ ] `harnez index` regenerates `issues/README.md` from `issues/*.md` +
      `issues/archive/*.md` metadata, matching current hand-written rows for
      the existing ticket set (no spurious diff on a clean run).
- [ ] `harnez index` regenerates `docs/README.md`'s studies table from
      `docs/studies/*.md`.
- [ ] Command is idempotent (`harnez index` twice produces no further diff).
- [ ] `go test ./...` passes.

## Notes

Keep scope tight to `issues/README.md` + `docs/studies/` first; the
`docs/lang|practices|other/*.md` tables carry hand-curated "Read
Trigger/Scope" phrasing that's harder to auto-derive and can be a follow-up
if the studies/issues pass proves out well.
