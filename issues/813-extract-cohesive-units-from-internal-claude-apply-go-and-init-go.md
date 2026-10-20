# 813 — Extract cohesive units from internal/claude apply.go and init.go

**Status**: Closed — Extracted cohesive apply/init units and replaced long parameter lists with options; behavior and required checks verified in 3c3ffc1d and b82ec106
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Refactor
**Related**: 807, 812

---

## 1. Problem & Motivation
`internal/claude/apply.go` (1985 lines) and `init.go` (1822 lines) are the next hotspots; `ApplyAllVariant` (479 lines, 8 params) and `RunInitWithVariant` (403 lines, 13 params) mix many concerns.

## 2. Technical Specification / Findings
Extract cohesive units (files and helper functions) with unit tests; no behaviour change.

## 3. Implementation & Verification Plan
`go test ./internal/claude` green, new unit tests for extracted helpers; hotspot numbers before/after.

## Handoff 2026-10-17
q-harnez (opus) stopped mid-turn when the Claude session limit was reached. Committed:
`7ddebdf8` (ApplyAllVariant phases). Uncommitted WIP left in the tree: `internal/claude/init.go`
(phase extraction) plus new `init_phases.go` and `init_phases_test.go`, untested. Next worker:
run `go test ./internal/claude`, finish or drop the init extraction, commit.

## Delivery 2026-10-20

- Reviewed committed phases `7ddebdf8` and `0d0839b7`: execution order,
  error wrapping, selection/removal gates, change counts and summary ordering
  match the original inline operations. Retained issue 821's bundled ownership
  exception in `removeDecommissionedArtifacts` unchanged.
- `3c3ffc1d`: extracted settings merging/reporting (`apply_settings.go`),
  artifact cleanup (`artifact_cleanup.go`), skill drift (`skill_diff.go`) and
  shell integration (`apply_shell.go`); added `ApplyOptions`/`InitOptions`.
- `b82ec106`: extracted canonical agent-file preparation and Git integration
  (`init_agent_file.go`), all-doc validation/variant resolution
  (`init_doc_preflight.go`), and opt-in quota scaffolding (`init_quota.go`).
  Replaced telescoping init entry points and all callers with options structs;
  CLI flags and behavior are unchanged. Existing test assertions remain intact.
- Metrics use `harnez assess --hotspots internal/claude`, supplemented by Go AST
  declaration spans (inclusive lines) and named-parameter counts, including
  grouped and variadic parameters. Baselines were also measured from the parents
  of the two WIP commits. Original → handoff → delivered:
  - `ApplyAllVariant`: 479/8 → 206/8 → **89 lines / 3 params**.
  - `RunInitWithVariant`: 403/13 → 266/13 → **114 lines / 3 params**.
  - `DiffAll`: 224/3 → 224/3 → **130 lines / 3 params**.
  - Next largest original functions: `installIssuesGitIntegration` **90/1**,
    `CleanAll` **89/2**, `isEligibleProjectDir` **71/1**, all unchanged.
  - Largest affected helper: `applySkills` **136/4**, unchanged from the handoff;
    new helpers include `diffSkills` **101/2**, `applyShellIntegration` **77/4**,
    `initAgentFile` **60/4**, and `initDocPreflight` **44/4**.
  - `RunInitAllWithVariant`: handoff **46/11** → delivered **48/3**.
  - File lines: `apply.go` 1985 → 1713 → **1108**;
    `init.go` 1822 → 1685 → **1495**.
- Stop point: all affected functions are below 150 lines; remaining orchestration
  is sequential. Unrelated package hotspots (`RunStatus`, plugin reconciliation)
  remain outside this ticket.
- Added unit assertions for dropped-skill ownership, drift and read errors;
  variant retention/override/fallback and preflight failures without writes;
  legacy agent-file migration and idempotency; managed-section pruning with
  sticky/user text preservation; doc/template copying and local-tail preservation.
- Passed `make lint` (warnings remain non-fatal),
  `go test ./internal/claude/ ./cmd/harnez/`, `make test-short`,
  `git diff --check`, and `make install`. Tests ran through quota-1 enforcement.
  Initial verification caught a local options-variable collision, corrected
  before the successful full verification; no code edits followed that run.
- Installed CLI has no `apply --check`; its dry equivalent is `harnez diff`.
  Captured that command before installation and after `make install` on this
  machine: stdout/stderr byte-identical (`cmp`). Compared old/new CLI init runs
  in the same temporary destination with `--force --yes --docs golang
  --variant lite --repo-mode solo`: fresh and repeat stdout/stderr, every generated
  file's SHA-256/mode, and symlink targets were identical. No golden updates.
- Process friction: quota-1's isolated runner cannot read a host `/tmp` script;
  the inline verification command worked. No agents or detached jobs started.
