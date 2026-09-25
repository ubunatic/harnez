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
