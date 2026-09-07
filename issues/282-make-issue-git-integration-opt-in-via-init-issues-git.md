# 282 — Make issue Git integration opt-in via init issues-git

**Status**: Closed — implemented by 3f663f8
**Priority**: P1 (High)
**Severity**: Major
**Category**: Feature
**Related**: [280](280-make-generated-issue-index-and-ticket-numbering-rebase-safe.md), `internal/claude/init.go`, commit `b9bf99d`

---

## 1. Problem & Motivation

Plain `harnez init` currently installs issue-specific Git integration as a side
effect: `.gitattributes`, the `harnez-issues-index` local merge driver, and the
issue-index pre-commit hook. This is surprising for projects that only want the
standard project setup, and can modify repository-local Git behavior without an
explicit opt-in. The behavior was introduced as part of issue 280 and is present
in `internal/claude/init.go` as of commit `b9bf99d`.

## 2. Technical Specification / Findings

Add a narrowly scoped `harnez init --issues-git` opt-in. Do not introduce a broad
`--git` switch. The default `harnez init` path must not silently install or modify
the issue integration files/configuration/hooks.

Define an idempotent disable/remove contract for the integration that:

- removes only harnez-managed attributes, merge-driver configuration, and hooks;
- preserves unrelated `.gitattributes` entries, Git configuration, and hooks; and
- behaves safely when run repeatedly or when some managed pieces are absent.

`harnez issues rebase` must fail early when issue Git integration is not enabled,
and its error must provide the exact setup command: `harnez init --issues-git`.

## 3. Implementation & Verification Plan

1. Separate issue Git integration from plain initialization behind
   `--issues-git`, including help and validation behavior.
2. Specify and implement idempotent enable/disable cleanup with ownership markers
   or equivalent narrow matching so unrelated user configuration survives.
3. Make `issues rebase` detect missing setup before attempting rebase operations
   and emit the exact setup command.
4. Add tests covering default init, opt-in init, repeated enable/disable, mixed
   unrelated Git state, and the early rebase failure message.
