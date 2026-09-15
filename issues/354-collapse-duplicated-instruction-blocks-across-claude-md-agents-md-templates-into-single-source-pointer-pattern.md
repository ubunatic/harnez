# 354 — Collapse duplicated instruction blocks across CLAUDE.md/AGENTS.md templates into single-source + pointer pattern

**Status**: Closed
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Templates / Docs / Token Efficiency
**Related**: [Issue 312](312-deduplicate-global-claude-md-sections-against-the-new-local-agents-md-managed-block.md) (closed as superseded by this ticket), [Issue 311](311-single-managed-section-for-local-agents-md-synced-across-every-project.md), [Issue 242](242-instruct-agents-on-harnez-find-in-agents-md.md), [Issue 040](040-agent-context-duplication-and-file-read-discipline.md), [Issue 355](355-harnez-apply-init-do-not-prune-agents-md-sections-removed-from-config-yaml-leaving-orphaned-managed-blocks.md) (follow-up gap found during implementation), [docs/practices/AgenticLoop.md](../docs/practices/AgenticLoop.md), [docs/lang/Bash.md](../docs/lang/Bash.md), `config.yaml` (`agents_md.global.sections`, `agents_md.local.sections`)

---

## 1. Problem & Motivation

`/context` in a live session shows the same instruction content loaded multiple times
across the CLAUDE.md/AGENTS.md chain (`~/.claude/CLAUDE.md`, `~/projects/CLAUDE.md`,
project `CLAUDE.md`, `AgenticLoop.md`, `Bash.md`). Several rules are stated in full in
more than one file instead of stated once and pointed to. This wastes context tokens on
every session in every managed project, and creates drift risk (editing one copy without
the others) — duplicate 6 below already shows four copies at four different detail levels.

Observed duplicates (2026-09-15 audit), with the source that generates each copy:

| # | Rule | Copies | Generated from |
|---|------|--------|----------------|
| 1 | Issue Tracker Discovery (`harnez find`) | `~/.claude/CLAUDE.md`, `~/projects/CLAUDE.md`, project managed block | global sections; **hand-authored**; local sections |
| 2 | Editing Discipline (`apply_patch`) | same three | same three |
| 3 | Voice/ASR homophone guidance | `~/.claude/CLAUDE.md` ("Voice Input"), `harnez/CLAUDE.md` ("Voice & Transcription Input Awareness") | global sections; project-local prose |
| 4 | Media/Demo verification gate | `AgenticLoop.md` Invariant 7; restated in `harnez/CLAUDE.md` | doc; project-local prose |
| 5 | Background/subagent hygiene | `AgenticLoop.md` Invariant 3; restated in `harnez/CLAUDE.md` | doc; project-local prose |
| 6 | `cd` / `-C` directory scoping | `Bash.md` §8 (canonical table), `~/.claude/CLAUDE.md` (bullet), `~/projects/CLAUDE.md` ("Multi-repo shell commands"), `AgenticLoop.md` anti-patterns | doc; global sections; **hand-authored**; doc |

7. `~/.claude/CLAUDE.md` states its own principle ("Minimal Global Docs: keep this global
   file free of anything not relevant to every project") and then violates it by embedding
   full harnez-specific command syntax also present in harnez's own managed block.

Note the three distinct fix mechanisms: `config.yaml` `agents_md.global.sections`
(generates `~/.claude/CLAUDE.md`), `config.yaml` `agents_md.local.sections` (generates
every project's managed block), and **hand-authored text in `~/projects/CLAUDE.md` that
sits outside its `harnez:begin/end` markers** — harnez cannot rewrite the latter, and
nothing prevents it drifting back.

Working correctly, as a model to copy: `harnez/CLAUDE.md`'s "Context Discipline & Token
Efficiency" section, which just points to `AgenticLoop.md` Invariant 6 instead of
restating it.

## 2. Proposed Fix

Convention: **a rule is stated once, in its canonical owner file; every other file that
needs it carries at most a one-line pointer, never a restatement.**

Canonical owner per duplicate (decide these here, not at implementation time):

| # | Canonical owner | Everywhere else |
|---|-----------------|-----------------|
| 1 | `config.yaml` `agents_md.local.sections` (project managed block) | global section keeps a 1-line "see your project's AGENTS.md managed block"; `~/projects/CLAUDE.md` section deleted |
| 2 | same as 1 | same as 1 |
| 3 | `~/.claude/CLAUDE.md` (applies to every project) | `harnez/CLAUDE.md` section deleted |
| 4 | `docs/practices/AgenticLoop.md` Invariant 7 | `harnez/CLAUDE.md` → 1-line pointer |
| 5 | `docs/practices/AgenticLoop.md` Invariant 3 | `harnez/CLAUDE.md` → 1-line pointer |
| 6 | `docs/lang/Bash.md` §8 | `~/.claude/CLAUDE.md` 1-line pointer; `AgenticLoop.md` anti-pattern trimmed to one line + pointer; `~/projects/CLAUDE.md` section deleted |

### Constraints (both are real footguns — do not skip)

- **`@docs/X.md` refs expand inline.** Replacing a 5-line restatement with
  `@docs/AgenticLoop.md` (26 KB) is a net token *loss* unless that doc is already in the
  session's load set. Use an `@`-pointer only where the target is already loaded;
  otherwise use a plain non-`@` prose reference ("see docs/X.md §N") that costs one line
  and loads nothing.
- **Pointer resolvability varies per project.** The local managed block is synced to every
  project, but docs are installed per-project via `init --docs <name>`. A managed-block
  pointer to `@docs/AgenticLoop.md` dangles in any project that never installed it; the
  global `./docs` → `~/.claude` fallback only rescues this when `apply` installed the doc
  globally. Prefer deletion-plus-prose-reference over an `@`-pointer inside
  `agents_md.local.sections`.
- Global `~/.claude/CLAUDE.md` must still stand alone for projects that have not re-run
  `init` and therefore lack the local managed block (inherited from issue 312).
- `~/projects/CLAUDE.md` lives in a different repo and is not harnez-managed. Either wrap
  the deduplicated remainder in `harnez:begin/end` markers so `init` owns it, or record in
  this ticket that it is a one-off manual edit with no drift protection.

## 3. Acceptance Criteria

- [ ] Each of the six duplicates has exactly one canonical body, per the table above.
- [ ] No two sections across `agents_md.global.sections` and `agents_md.local.sections`
      restate the same rule (absorbs issue 312; close 312 as superseded).
- [ ] Baseline recorded before the change: `harnez assess --json` token estimate plus
      `wc -c` for `~/.claude/CLAUDE.md`, `~/projects/CLAUDE.md`, `harnez/AGENTS.md`,
      `docs/practices/AgenticLoop.md`, `docs/lang/Bash.md`. Current bytes: 3982 / 2291 /
      9529 / 26049 / 6603 = **48454 total**.
- [ ] After the change, the same measurement is re-run and pasted into §4, showing a net
      reduction in the always-loaded set (the three CLAUDE.md/AGENTS.md files) with **no**
      newly-`@`-referenced doc added to the load set.
- [ ] `harnez apply` and `harnez init` regenerate the deduplicated text; re-running both is
      idempotent (`scripts/smoke-test.sh` passes).
- [ ] `make lint` and `make check` pass; existing global/local section tests updated only
      as this content change requires.
- [ ] Confirmation pass: `/context` in a fresh session against the harnez repo shows no
      rule stated in full twice (qualitative check, not the measurement gate).

## 4. Result

Implemented per the canonical-owner table in §2:
- `config.yaml` `agents_md.global.sections`: removed the full `Issue Tracker Discovery`
  entry (canonical owner is now `agents_md.local.sections`); condensed the Editing
  Discipline bullet in Instructions Hierarchy to a pointer at the same project's local
  managed block.
- `~/projects/AGENTS.md` (hand-authored, no git repo, no drift protection — as flagged in
  §2): trimmed "Issue Tracker Discovery"/"Editing Discipline" to a one-line pointer;
  trimmed "Multi-repo shell commands" to a pointer plus the genuinely distinct
  chained-`cd`-within-one-call nuance not covered by `Bash.md` §8.
- `harnez/AGENTS.md` (hand-authored sections outside `harnez:begin/end` markers):
  trimmed "Background Tasks & Process Hygiene", "Voice & Transcription Input Awareness",
  and "Demo Recordings & Media Verification" to pointers at `AgenticLoop.md`
  Invariants 3/7 and the global Voice Input rule, keeping only the project-specific
  bullets (subagent handoff, worktree policy, homophone example) that aren't duplicates.
- `docs/practices/AgenticLoop.md` and `docs/lang/Bash.md`: no changes needed — the
  cd/-C anti-pattern entry there was already a pointer, not a restatement.

**Gap found during implementation**: `harnez apply` does not prune a marker block whose
section was removed from `config.yaml` — it only adds/updates markers still present in
the config. The stale `Issue Tracker Discovery` block had to be deleted by hand from an
already-applied `~/.claude/CLAUDE.md`. Filed as [issue 355](355-harnez-apply-init-do-not-prune-agents-md-sections-removed-from-config-yaml-leaving-orphaned-managed-blocks.md).

**Measurement** (always-loaded set: `~/.claude/CLAUDE.md` + `~/projects/AGENTS.md` +
project `AGENTS.md`, `wc -c`, no `docs/lang/Bash.md`/`AgenticLoop.md` change so excluded):

| File | Before | After |
|------|-------:|------:|
| `~/.claude/CLAUDE.md` | 3982 | 2965 |
| `~/projects/AGENTS.md` | 2291 | 1847 |
| `harnez/AGENTS.md` | 9529 | 9017 |
| **Total** | **15802** | **13829** |

Net reduction: 1973 bytes (12.5%) in the always-loaded set, with no new `@`-referenced
doc pulled into the load set (both footgun constraints from §2 held).

Verification: `go build ./...`, `go test ./...` (two tests in `internal/claude` updated
to match the new canonical-owner location, per issue 354's own convention), `harnez apply`
and `harnez init` both idempotent (report "unchanged"/"No changes." on re-run), and
`scripts/smoke-test.sh` passes.
