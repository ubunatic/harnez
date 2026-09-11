# 311 — Single managed section for local AGENTS.md, synced across every project

**Status**: Closed
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics / Docs Pipeline
**Related**: `docs/templates/AGENTS.md`, `config.yaml` (`agents_md.global`, `agents_md.local`), `internal/claude/apply.go` (`cleanSectionMD`), [Agentic Loop Practices](../docs/AgenticLoop.md)

---

## 1. Problem & Motivation

There is recurring confusion — for agents and for the user — between harnez's own
local `AGENTS.md` (hand-authored rules for working in *this* repo) and the
copyable/managed docs and templates harnez distributes to other projects
(`docs/templates/AGENTS.md`, `docs/lang/`, `docs/practices/`, etc.), because both
live in the same repository.

Part of why the confusion recurs: universal, cross-cutting rules that should be
identical everywhere are currently duplicated by hand across at least three places
with no sync mechanism between them:
- `config.yaml`'s `agents_md.global.sections` ("Instructions Hierarchy" section),
  which generates `~/.claude/CLAUDE.md`.
- `docs/templates/AGENTS.md`, the scaffold `init` writes into every new project's
  local `AGENTS.md`.
- Harnez's own project `AGENTS.md`, which further diverged from the template with
  extra hand-added sections.

Concretely: "Editing Discipline" is byte-identical text living independently in the
global config content and in the template, and harnez's own `AGENTS.md` inherited
it unchanged from the template, plus separately duplicated `AgenticLoop.md`'s
"Zero Zombie Guarantee" (in an expanded "Background Tasks & Process Hygiene"
section) and Invariant 7 (in a "Demo Recordings & Media Verification" section)
instead of pointing to them — inconsistent with harnez's own file's "Context
Discipline & Token Efficiency" section, which already does this correctly via a
one-line pointer to `@docs/AgenticLoop.md`.

Global `~/.claude/CLAUDE.md` already solves exactly this class of problem for
itself: it defines named `sections:` in `config.yaml`, each wrapped in
`harnez:begin`/`harnez:end` markers, so `apply` can safely rewrite just those
blocks while nothing else in the file is touched. Local `AGENTS.md` has no
equivalent — `docs/templates/AGENTS.md` only wraps two unrelated blocks this way
(`Local Overlays`, for the `@AGENTS.local.md` reference managed by `mode`; `Project
Summary`, managed by `init --summary`) — so there is no way to push a rules update
into every already-`init`'d project's `AGENTS.md` without a full `--replace` (which
would also wipe out each project's local customizations).

**Design decision from discussion** (see session context): exactly **one** new
managed section, not several topic-based sections mirroring the global file's four.
The universal rules are short and thematically related ("stuff harnez keeps synced
across every project"), so one block is simpler to reason about, diff, and
reconcile on re-apply than many.

## 2. Scope & Technical Requirements

- Add exactly one new managed section (name TBD, e.g. `Harnez Managed Conventions`)
  under `agents_md.local` in `config.yaml`, following the same section/content
  model already used for `agents_md.global.sections`.
- Wrap it in `docs/templates/AGENTS.md` with its own distinct
  `harnez:begin`/`harnez:end` marker pair — **do not** fold it into the existing
  `Local Overlays` or `Project Summary` blocks; those serve different, unrelated
  purposes and must stay as they are.
- Content of the new section: only rules that should be identical in literally
  every project. Starting candidates: Editing Discipline (currently duplicated
  verbatim in both the template and the global file), Issue Tracker Discovery
  (`harnez find`/`harnez issues`), and a pointer to relevant `docs/AgenticLoop.md`
  invariants instead of restating them. Final content list is an implementation
  call, not fixed by this ticket.
- Wire `harnez init` (and/or `harnez apply`, and/or a new sync command — TBD at
  implementation time) to write/refresh only this one marked block in an
  **existing** `AGENTS.md`, leaving all other content — including project-specific
  custom sections — untouched. Reuse/generalize the existing `cleanSectionMD`
  section-rewrite primitive (`internal/claude/apply.go`) already used for global
  `CLAUDE.md`'s managed sections, rather than inventing a new mechanism.
- Migrate already-`init`'d projects, **including harnez's own project `AGENTS.md`**:
  replace their existing free-form/hand-duplicated copies of the now-managed
  content with the new marked block, without ending up with both a managed copy
  and a stale hand-written copy side by side.
- Harnez's own `AGENTS.md` keeps its project-specific extra sections (`Demo
  Recordings & Media Verification`, `Voice & Transcription Input Awareness`, the
  expanded `Background Tasks & Process Hygiene`) as free-form custom content
  outside the managed block, unless a future ticket decides some of these are
  universal enough to fold in too. Not this ticket's call.
- Out of scope, explicitly: the user is separately adding a disclaimer/pointer
  note to harnez's own `AGENTS.md` about the existence of `docs/templates/AGENTS.md`.
  That note is local to harnez only and must **never** be copied into
  `docs/templates/AGENTS.md` itself — other projects' generated `AGENTS.md` files
  must carry zero mention of harnez's internal template/self-management mechanics.
  Implementation of this ticket must not clobber or relocate that note once added.
- **Not in scope** (retracted during discovery, no action needed): `docs/AgenticLoop.md`
  (repo root) vs. `docs/practices/AgenticLoop.md` looked like an accidental
  duplicate but is not — `config.yaml:555-559` declares the root copy as an
  intentional `local:` target that `docs_capture.go` uses for drift detection
  against the canonical source. Leave it alone.

## 3. Acceptance Criteria

- [x] Exactly one new managed section defined under `agents_md.local` in
      `config.yaml`, content-sourced the same way `agents_md.global.sections` is.
- [x] `docs/templates/AGENTS.md` carries the new section wrapped in its own
      `harnez:begin`/`harnez:end` markers, distinct from `Local Overlays` and
      `Project Summary`.
- [x] The section's content contains only universal, cross-cutting rules — no
      project-specific content.
- [x] "Editing Discipline" no longer exists as unmarked, hand-duplicated free text
      in `docs/templates/AGENTS.md` — it lives only in the new managed section (or
      is otherwise fully deduplicated against the global file's copy — final call
      at implementation time).
- [x] A sync mechanism (via `init`/`apply` or a new command) rewrites just this
      block in an existing `AGENTS.md` without disturbing any other content,
      reusing the existing `cleanSectionMD`-style primitive.
- [x] Harnez's own project `AGENTS.md` is migrated: its Editing Discipline (and any
      other now-managed content) comes from the new managed block, not
      hand-duplicated text; its project-specific extra sections remain as
      free-form custom content, untouched.
- [x] `docs/templates/AGENTS.md` contains no mention of harnez's own internal
      template/self-management mechanics — that context stays local to harnez's
      own `AGENTS.md` only, and is not authored by this ticket's implementation.
- [x] No unrelated docs/config/generated files are modified beyond this scope.

## 4. Migration & Verification Plan

1. Inspect `config.yaml`'s `agents_md.global` and `agents_md.local` blocks and
   `internal/claude/apply.go`'s `cleanSectionMD` to confirm the exact mechanism to
   generalize.
2. Add the new managed section to `agents_md.local` and to
   `docs/templates/AGENTS.md`; wire the sync path.
3. Dry-run against harnez's own project first (`make apply`/`make init`-equivalent
   or the new sync path), verify idempotency (a second run reports no changes),
   and confirm harnez's own `AGENTS.md` ends up with the managed block plus its
   pre-existing custom sections intact and unduplicated.
4. Spot-check against at least one other already-`init`'d sibling project (if one
   is available) to confirm the migration doesn't require `--replace` and doesn't
   clobber that project's local customizations.
5. `git diff`/`git status` to confirm scope: only the intended config/template/code
   files plus the migrated `AGENTS.md` files changed.

## 5. Implementation Notes

- The mechanism already existed end-to-end (`AgentsMDTarget.Sections`, `init.go`'s
  `applySectionMD` loop, `status.go`'s presence checks, `docs_capture.go`'s drift
  compare) — no schema change was needed, just config content, a template edit, and
  a migration.
- **Real bug found and fixed**: `DiffAll` (`internal/claude/apply.go`) unconditionally
  checked `agents_md.local.Sections` for drift, but `ApplyAll` never writes them —
  that's exclusively `init`'s job, per `buildAgentProfileTestConfig`'s own doc comment
  in `agents_profile_test.go`. This was latent dead code (harmless while
  `local.sections` was empty) that broke six unrelated tests the moment content was
  added, since `agents_md.local.Target` is the relative path `AGENTS.md` — resolved
  against whatever the process's CWD happened to be, not the target under diff.
  Removed the check from `DiffAll` rather than making `ApplyAll` write local sections,
  per this repo's own CLI-scope-separation rule (`apply` is global-only, `init` is
  project-only). Added `TestRunInit_AppliesManagedConventionsSection` for regression
  coverage of the intended (correct) path.
- Migration of harnez's own `AGENTS.md` was a one-time hand-edit, not something
  `init` can do automatically — `markdown.Apply`/`applySection` only replaces content
  between existing markers or appends at EOF; it never removes unmarked free text.
  Confirmed via `internal/markdown/markdown.go`. This means any other already-`init`'d
  project that had the same free-text duplication needs the same one-time manual
  cleanup; the new managed block will otherwise just land appended at the end of its
  `AGENTS.md` on next `init`, coexisting with (not replacing) the stale free text.
- **Follow-up filed as issue 312**: `config.yaml`'s `agents_md.global.sections`
  ("Instructions Hierarchy") still independently hand-duplicates "Editing Discipline"
  and part of "Issue Tracker Discovery", and the two copies have since drifted to
  different wording — so the de-duplication this ticket set out to fix is 2-of-3, not
  3-of-3. Acceptable per this ticket's own AC #4 (explicitly left as an implementation
  call), but worth closing separately.
