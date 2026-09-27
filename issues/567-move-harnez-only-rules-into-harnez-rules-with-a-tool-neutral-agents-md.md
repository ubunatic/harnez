# 567 — Move harnez-only rules into .harnez/rules/ with a tool-neutral AGENTS.md

**Status**: Open
**Priority**: P1
**Severity**: Medium
**Category**: Init / Agent Instructions
**Related**: [[568-split-issuetracking-agenticloop-and-gorelease-into-generic-docs-and-harnez-rules]], [[539-agents-in-other-projects-don-t-know-harnez-agent-or-model-names-like-terra-low]]

---

## Problem

`harnez init` writes five managed blocks (Local Overlays, Harnez Managed Conventions,
Repo Setup, Language Conventions, Quota-1 Guardrails) into `AGENTS.md`, interleaved with
the owner's rules. The file is neither tool-neutral nor single-owner, and needs a
"put your rules outside the blocks" rule to protect hand edits. `AGENTS.local.md` holds
further managed blocks (Concise Mode, Subagent Policy).

## Design

Sorting rule: would the rule still make sense with harnez uninstalled? Yes → `docs/`.
No → `.harnez/rules/`.

```
AGENTS.md                owner's rules, tool-neutral; only a short managed header
docs/                    generic practices (languages, git, loop, issue format)
.harnez/rules/Index.md   generated entry point listing the rule files; committed
.harnez/rules/*.md       harnez-only rules, fully generated, never hand-edited
.harnez/rules/Local.md   per-checkout toggles, git-excluded (replaces AGENTS.local.md)
```

`.harnez/` in a project holds rules only, no runtime state.

Header in `AGENTS.md`:

```markdown
**Before any work, read `.harnez/rules/Index.md` if it exists, then
`.harnez/rules/Local.md`; they are part of this file. Local.md overrides both.**
```

## Scope (first pass)

Move clearly harnez-only content into rule files (PascalCase names):

- `Tools.md` — harnez install, `harnez read`, editing discipline
- `Issues.md` — `harnez find` / `harnez issues` usage
- `Quota.md` — Quota-1 guardrails
- `Subagents.md` — `harnez agent`, roles, models, subagent policy text
- `Output.md` — ConciseMode tiers, `/mode`

Out of scope: IssueTracking, AgenticLoop and GoRelease stay in `docs/` unchanged
(they mix generic practice with harnez commands; split in 568).

## Subagent modes

Add the `mixed` mode designed in `docs/HarnezComponents.md` §8.9 and make it the default:

| Mode | Dispatch |
|---|---|
| `native` | host's own subagents only |
| `mixed` (default) | native for the host's vendor, `harnez agent --model <spec>` for other vendors or a named model ("ask luna:low") |
| `harnez` | every subagent through `harnez agent` |

Today (83e90bb) the `native` policy text already describes `mixed`; restore `native` to
native-only once `mixed` exists. `Subagents.md` / `Local.md` carry the mode text; with the
`agents` component disabled the effective mode is `native`.

## Tasks

- `init` generates `.harnez/rules/` and the `AGENTS.md` header; `Local.md` added to
  `.git/info/exclude`.
- `agent` gets a `mixed` setting (e.g. `harnez agent mode native|mixed|harnez`); `enable/disable` and `/mode` write `.harnez/rules/Local.md` instead of `AGENTS.local.md`.
- Migration: `init` removes the old managed blocks from `AGENTS.md`, moves
  `AGENTS.local.md` blocks into `Local.md`, keeps owner text untouched.
- Update `docs/CLIDesign.md` and the CLAUDE.md "Where Repo Rules Go" section.
- Tests for migration idempotency; run on this repo and one other managed project.

## Milestones (product owner, 2026-09-27, terra plan)

Hard dependency: **355** (prune removed managed blocks) lands first. 471 is sequencing debt for 568, not a
blocker here. The `mixed` subagent mode moves to **493**; 567 only migrates existing `native`/`harnez` values.

Code map: init creates/migrates AGENTS.md `internal/claude/init.go:742-764`, Local Overlays `:457-480,
:778-785`, configured sections `:878-905`, Quota `:980-986`; content `config.yaml:485-570`; section primitives
`internal/claude/apply.go:30-45`, `internal/markdown/markdown.go:254-265`; agent policy
`cmd/harnez/agent.go:559-581`, `internal/agentpolicy/policy.go:53-77`; `/mode` `internal/mode/mode.go:122-177`.

- **M1** generate `.harnez/rules/{Index,Tools,Issues,Quota,Subagents,Output}.md` and the AGENTS.md header; add
  `.harnez/rules/Local.md` to `.git/info/exclude` (skip cleanly outside git). Tests: clean init, idempotent.
- **M2** lossless migration: move recognized managed blocks out of AGENTS.md and known AGENTS.local.md
  sections into rules/Local.md, prune via 355; unmatched owner bytes unchanged. Fixture: two runs equal;
  malformed or nested markers are left untouched with a warning.
- **M3** `/mode` and `agent enable|disable` read/write `.harnez/rules/Local.md`, still read AGENTS.local.md once
  to migrate. Tests for both writers.
- **M4** config/templates, `docs/CLIDesign.md`, CLAUDE.md "Where Repo Rules Go", self-init of this repo; canary
  `harnez init` on a temp copy of one other managed repo (not the live repo); assert no AGENTS.local.md
  dependency remains.

M1 delivered

M2 policy choice: `Local Overlays` is removed because the new header supersedes its `AGENTS.local.md` instruction; `Project Summary` stays in `AGENTS.md` because it is project-specific context rather than a Harnez rule.

M2 refinements: `Language Conventions` (the `docs/*.md` index) and opt-in doc references stay in `AGENTS.md` because they are tool-neutral. Lite Quota-1 media-gate guidance moves with the Quota-1 block into `.harnez/rules/Quota.md`. The Subagent Policy value is preserved exactly; only its location moves into `.harnez/rules/Local.md`.

M2 delivered

### M2 Pre-Work / Required Refinements (host review 2026-09-27)
- This repo's `.gitignore` ignores `.harnez/`, so the committed rules needed a force-add. Narrow it so
  `.harnez/rules/*.md` is tracked and `.harnez/rules/Local.md` stays excluded; init should warn (not edit
  `.gitignore`) when a repo ignores `.harnez/rules/`.
- M1 canary on clones of loom, voxi, neus, cati: no block removed, rules generated, not ignored. Re-run the
  same canary for M2 (host does it at review): owner text byte-identical outside managed blocks.

### M3 Pre-Work / Required Refinements (host canary 2026-09-27)
- M2 canary on clones of loom, voxi, neus, cati, lmcoder: owner text byte-identical, second run idempotent,
  Quota-1 content in Quota.md. **Bug:** voxi's `Repo Setup` block lands in `.harnez/rules/Local.md`, which is
  git-excluded, so a durable repo rule drops out of version control. `Repo Setup` is tool-neutral (branch and
  push policy): it stays in AGENTS.md like Language Conventions. Fix first, with a test.

M3 refinement resolved: `Repo Setup` remains in `AGENTS.md`; it is tool-neutral branch/push policy, not a local toggle.

M3 delivered

## M3 Review (host, 2026-09-27)
- M3 delivered (f02b602): Repo Setup stays in AGENTS.md, Local.md drives /mode and agent enable|disable.
- Clone canary (loom, voxi, neus, cati, lmcoder): owner text byte-identical, voxi Repo Setup kept, idempotent except cati.

### M4 Pre-Work / Required Refinements
- cati is not idempotent: first `init` appends newly added opt-in docs (Make, Markdown, Spec) at the end of the
  Language Conventions list; the second run sorts them in. Emit the sorted order on the first write. Add a test:
  init twice on a repo gaining docs, second run is a no-op.

M4 pre-work resolved: canonicalize selected docs to config order when writing `Language Conventions`; regression verifies a repo gaining Make, Markdown, and Spec has byte-identical `AGENTS.md` after its second init.

M4 delivered
