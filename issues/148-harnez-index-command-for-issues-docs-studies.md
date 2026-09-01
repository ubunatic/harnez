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
