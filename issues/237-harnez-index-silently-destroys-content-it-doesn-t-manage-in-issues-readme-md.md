# 237 — `harnez index` silently destroys content it doesn't manage in `issues/README.md`

**Status**: Open
**Priority**: P1 (High)
**Severity**: Critical
**Category**: Bug
**Related**: `internal/index/index.go` (`UpdateIssuesReadme`), `cmd/harnez/index.go`,
`cmd/harnez/issues.go` (`harnez issues <verb>`'s default commit-and-resync path also calls
`UpdateIssuesReadme`), `docs/practices/IssueTracking.md` §4.1 (Index Table — the documented
contract this violates), [[232-harnez-issues-verb-command-for-single-call-status-changes-with-index-sync-and-commit]]
(introduced the second call site that hits this same bug)

---

## 1. Problem & Motivation

`harnez index -d <repo>` (and `harnez issues <verb>`'s default resync-and-commit path, which
calls the same function) can silently overwrite hand-authored content in `issues/README.md`
that has nothing to do with the ticket table, with no warning, no diff shown by default, and no
preservation mechanism — unlike `harnez init`'s `AGENTS.md` handling, which uses
`<!-- harnez:begin/end -->` markers specifically so it never touches content outside its own
managed block.

### Repro (observed tonight, not committed — caught in review)

A subagent ran `harnez init` + `harnez index -d <repo>` against two sibling repos to verify the
tooling against their pre-existing issue trackers:

- **trafficsim**: `issues/README.md` originally had a **Priority** column in its table, plus
  ~200 lines of hand-authored prose around/below the table (priority-tier rationale, a "WebApp UI
  track" section, and two ordered "recommended work queue" / "project plan audit" roadmap
  sections). After `harnez index`, the file was regenerated using harnez's bare
  `# | File | Title | Status` table format — the Priority column and **all ~200 lines of prose
  were gone**.
- **`.workspace`**: same pattern — a **Target** column and a "Global Inbox Guidelines" section
  were deleted.

The subagent caught this via `git diff` before committing and ran `git checkout --
issues/README.md` in both repos to discard the damage. No real data was lost this time only
because a human/agent review step happened to precede the commit — the tool itself has no
safeguard.

## 2. Root Cause

`internal/index/index.go`, `UpdateIssuesReadme` (~line 119-153):

```go
loc := issuesTableHeaderRe.FindStringIndex(content)
...
newContent := content[:loc[0]] + table
```

This preserves every line **before** the `| # | File | Title | Status |` header line verbatim,
but replaces everything from that header line to **end of file** with the freshly generated
canonical table — unconditionally. Any prose sections placed after the table, and any extra
columns baked into the table itself (since `table` is always harnez's own canonical bare-column
format via `IssuesTable()`), are destroyed rather than merged or preserved.

This contradicts the documented contract in `docs/practices/IssueTracking.md` §4.1, which
describes `harnez index` as regenerating "its table" — implying only the table, not the rest of
the file — and says manual edits are safe until the next run "overwrites" them, without
disclosing that the whole tail of the file past the header is in scope for that overwrite.

## 3. Scope

- Fix `UpdateIssuesReadme` so it no longer discards content structurally outside the table it
  manages (extra columns and/or trailing prose sections).
- Applies to both call sites: `harnez index` (`cmd/harnez/index.go`) and `harnez issues <verb>`'s
  default resync-and-commit path (`cmd/harnez/issues.go`, from ticket 232).
- Out of scope: `UpdateDocsReadme` (`internal/index/index.go` ~line 261) has the same
  `content[:loc[0]] + table` shape for `docs/README.md`'s studies table — worth checking whether
  it has the identical bug, but not fixing it here; file separately if confirmed rather than
  silently expanding this ticket's scope.

## 4. Open Questions (fix design intentionally not prescribed)

Two candidate approaches surfaced during investigation, precedent exists in this codebase for
the first but not clearly for the second — pick based on what's simplest to implement correctly,
not listed in preference order:

1. **Marker-based preserve**, mirroring `AGENTS.md`'s `<!-- harnez:begin/end -->` convention:
   only ever rewrite content between explicit markers (auto-inserted around the table on first
   run), leaving everything else — including a wider/differently-columned table a project has
   customized — untouched.
2. **Refuse-and-warn on unrecognized shape**: if the table found doesn't match harnez's exact
   canonical column set, or non-blank content exists after the table's last row, error out (or
   require `--force`) instead of silently overwriting, forcing a human/agent to reconcile by
   hand once.

Either must also address: does `harnez index` currently have any way to preserve a
project-specific extra table column (e.g. Priority, Target) across regenerations, or does
adopting `harnez index` inherently mean giving up custom index columns? If the latter, that
constraint needs to be stated up front in `docs/practices/IssueTracking.md` §4.1 and/or the
`harnez init` onboarding flow, not discovered by users mid-data-loss.

## 5. Acceptance Criteria

- [ ] Running `harnez index` (or `harnez issues <verb>`'s resync) against a `issues/README.md`
  with content structurally outside harnez's managed table (extra columns, and/or prose before
  or after the table) either preserves that content, or refuses to write and reports why —
  never silently deletes it.
- [ ] Regression test using a fixture `issues/README.md` shaped like trafficsim's (extra column
  + trailing prose sections) asserting the non-table content survives a `harnez index` run (or
  that the run refuses and leaves the file untouched, per whichever design is chosen).
- [ ] `docs/practices/IssueTracking.md` §4.1 updated to state the actual contract precisely
  (what is/isn't safe to hand-author around the table) once the fix's behavior is decided.

## 6. Related, Smaller Finding (same investigation, not this ticket's scope)

`trafficsim`'s own `docs/README.md` lacked a section anchor `harnez index` expects for the
`docs/studies/` table regen, producing a harmless but confusing error on first run. Not filed
separately — worth a one-line diagnostic/error-message improvement if anyone hits this again,
but not worth its own ticket at this time.
