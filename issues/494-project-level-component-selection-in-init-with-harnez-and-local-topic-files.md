# 494 — Project-level component selection in init with harnez and local topic files

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: 490, 491, 492, `docs/HarnezComponents.md` §8.9, `docs/CLIDesign.md`

## /goal

Give `init` its own project-level selection, focused on (mostly) agent-agnostic docs
profiles and doc subsets, with per-project component steering in `<topic>.harnez.md` and
`<topic>.local.md` files and a commit policy chosen by repo type.

## 1. Problem & Motivation

`apply`'s component selection is global. Projects differ: a public repo should not force
harnez conventions on contributors, a solo repo may want them committed. Today `init`
writes managed AGENTS.md blocks that name `harnez find`, `harnez read`, and the rate
protocol regardless of project type.

## 2. Technical Specification

- `init` selection: docs profiles and subsets (existing `docs_profiles`, `--docs`), kept
  separate from `apply`'s components (CLIDesign separation stays load-bearing).
- Separation rule:
  - AGENTS.md and committed docs: agent- and tool-agnostic.
  - `<topic>.harnez.md`: anything naming harnez components or commands (dispatch mode,
    expected components, harnez tool usage).
  - `<topic>.local.md`: personal/machine overrides; always git-excluded (as
    `AGENTS.local.md` today).
- Commit policy per repo type, chosen at `init` and recorded:

  | Repo type | `*.harnez.md` | `*.local.md` |
  |---|---|---|
  | solo | committed | excluded |
  | team | committed | excluded |
  | public | excluded / out of tree | excluded |
  | custom | per-file choice | excluded |

- Move harnez-specific managed blocks (issue tracker commands, `harnez read`, rate
  protocol) out of AGENTS.md into `AGENTS.harnez.md`, linked from AGENTS.md.

## 3. Implementation & Verification Plan

- [ ] Decide topic set (AGENTS only first, or per-topic files)
- [ ] Repo-type flag/prompt on `init`, recorded for later runs
- [ ] Split managed blocks; git-exclude handling per policy
- [ ] Migration for existing projects (blocks move once, idempotent)
