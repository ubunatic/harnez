# 333 — Retire docs/feedback/, consolidate into docs/studies/ with labels/categories for grouping and filtering

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Refactor
**Related**: `docs/practices/AgenticLoop.md` Invariant 5 (In-Repository Single Source of
Truth, names both dirs as valid retrospective destinations), `docs/CLIDesign.md`
(copyable-doc contract names both dirs), `commands/sprint.md`/`commands/lean-sprint.md`
(Phase 5 retrospective step names both dirs), issue 332 and
`docs/studies/2026-09-13-cli-invocation-log-exit-code-anti-pattern-and-self-pollution.md`
(prompted this ticket — a session retrospective that didn't cleanly fit either directory)

---

## 1. Problem & Motivation

Project owner's framing: *"docs/feedback is not really used, docs/studies is all we
need — we'd rather add labels/categories to the index/files there to group and
filter."*

The immediate trigger: while writing today's session retrospective (issue
326/327/328's `harnez log` arc, the `os.Exit`-in-`RunE` fix, and the
`TestGearMulticallExecution` DB-pollution bug), the content was a genuine mix of a
technical case study (a bug and its fix) and agentic-process retrospective material
(subagent dispatch effectiveness, when to skip a reviewer, a live-vs-unit-test-only
lesson) — exactly the kind of content the project's own docs currently split across two
directories with no written rule for which one wins when content straddles both. The
file landed in `docs/studies/` by guesswork, not by policy.

## 2. Findings

- **`docs/studies/` has real tooling; `docs/feedback/` has none.** `internal/index/index.go`'s
  `UpdateDocsReadme` regenerates docs/README.md's studies table from `docs/studies/*.md`
  automatically (topic extraction via `StudyTopic`: a `<!-- harnez:topic: ... -->` override,
  else a `**Scope**:` line, else the H1) — its own doc comment explicitly says it "leaves
  every other table (root docs, lang, practices/other, **feedback**, proposed) untouched."
  `docs/feedback/`'s table in `docs/README.md` is hand-maintained prose with no generator,
  no topic-extraction convention, and no `harnez index` integration at all.
- **`docs/feedback/` has 12 files**, dated 2026-08-18 through 2026-09-11 — real historical
  content, not an empty/abandoned directory, so "remove" here means "stop treating it as an
  active destination and fold its role into `docs/studies/`," not delete the retrospective
  value those files carry.
- **Both directories are referenced from load-bearing places that all currently say "docs/feedback/
  or docs/studies/" without any decision rule**: `docs/practices/AgenticLoop.md` Invariant 5
  (and its generated copy `docs/AgenticLoop.md`), `docs/CLIDesign.md`'s copyable-doc contract,
  and both `commands/sprint.md` and `commands/lean-sprint.md`'s Phase 5 retrospective step. Any
  fix has to update all of these consistently, not just the two README tables.
- **No labels/categories convention exists yet on `docs/studies/` files.** Today the only
  per-file metadata `StudyTopic` reads is the optional `<!-- harnez:topic: ... -->` override
  and the `**Scope**:` line — neither is structured for filtering (e.g. "show me only
  agentic-process retrospectives" vs. "show me only technical bug case studies").

## 3. Proposed scope (not yet decided — filed for triage, per the owner's request to record the idea, not implement it directly from a slash-command filing session)

- **Retire `docs/feedback/` as a distinct destination.** Options for the 12 existing files:
  move them into `docs/studies/` with a `harnez issues mv`-style rename that preserves dates
  (matching the `YYYY-MM-DD-slug.md` convention `docs/studies/` already uses), or leave them
  in place under a redirect note and only change the *forward-looking* guidance. Prefer the
  former for a single source of truth, but confirm no external links depend on the old paths
  first (grep found no `docs/feedback/` links outside this repo's own docs/issues/commands/tests).
- **Design a labels/categories scheme for `docs/studies/`.** Likely shape: an extension to the
  existing `<!-- harnez:topic: ... -->` comment convention, e.g. a sibling
  `<!-- harnez:tags: agentic-process, bugfix -->` line, parsed by a small addition to
  `internal/index/index.go` (a new `StudyTags` alongside `StudyTopic`) and rendered as an extra
  column or a filterable grouping in the regenerated `docs/README.md` table. Needs a small,
  fixed vocabulary (open question: freeform tags vs. a closed enum like `docs/IssueTracking.md`'s
  Category field) — this is real design work, not a one-line change, and should probably get
  its own advisor pass given the "closed set vs. freeform" tradeoff mirrors issue 318's own
  "load-bearing taxonomy" concern.
- Update `docs/practices/AgenticLoop.md` (and its generated copy), `docs/CLIDesign.md`, and
  both sprint command docs to name `docs/studies/` alone once the migration lands, replacing
  every "docs/feedback/ or docs/studies/" phrasing.
- Update `internal/index/index.go`'s `UpdateDocsReadme` doc comment (currently says it leaves
  "feedback" untouched — that sentence becomes stale once the table is gone) and
  `internal/claude/docs_test.go`/`internal/index/index_test.go`'s feedback-table-related
  assertions.
- Decide whether copyable-doc dependents that currently name `docs/feedback/` as an optional
  destination (`docs/CLIDesign.md`'s copyable-doc contract line) need a fallback story for
  projects that installed docs referencing the old path via `init --docs`.

## 4. Verification

- [ ] `docs/feedback/` no longer exists as a directory (or is redirect-only, per whichever
      migration option is chosen), and its 12 files' retrospective content is preserved
      somewhere under `docs/studies/`.
- [ ] `docs/README.md` has one destination table (studies), fully regenerated by `harnez index`
      with no hand-maintained feedback section left over.
- [ ] A labels/categories scheme is designed and documented (even if the vocabulary starts
      small), with at least one real file tagged as a proof of concept.
- [ ] `docs/practices/AgenticLoop.md`, `docs/AgenticLoop.md`, `docs/CLIDesign.md`,
      `commands/sprint.md`, and `commands/lean-sprint.md` all name `docs/studies/` alone,
      with no remaining "docs/feedback/ or docs/studies/" phrasing.
- [ ] `internal/index/index.go`'s doc comments and `internal/claude/docs_test.go`/
      `internal/index/index_test.go` are updated to match; `go test ./...` passes.
- [ ] `make lint` passes (registered command/doc consistency).
