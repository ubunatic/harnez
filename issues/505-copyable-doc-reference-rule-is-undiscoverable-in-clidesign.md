# 505 — Copyable-doc reference rule is undiscoverable in CLIDesign

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Documentation / DX
**Related**: 249 (copied practice docs retain dangling dependencies), `docs/CLIDesign.md` ("Copyable-doc contract"), `internal/claude/docs_test.go`, `CLAUDE.md` ("Docs Layout")

## /goal

An agent about to edit a doc under `docs/practices/`, `docs/lang/` or `docs/other/` learns the
copyable-doc reference rule before it writes, not from a failing `internal/claude` test
afterwards.

## Problem

The rule — a copyable doc must not leave repository-relative links such as
`docs/studies/…` or `docs/feedback/…`, because they only resolve in harnez's own tree — is
documented, in `docs/CLIDesign.md` under "Copyable-doc contract". That file's stated purpose in
`CLAUDE.md` is the `apply`/`init` command separation, so nobody editing a practice doc thinks to
open it.

Observed 2026-09-22: a host and two `claude:sonnet:low` workers all added and reviewed a new
section in `docs/practices/ModelRoles.md` citing two `docs/studies/` and `docs/feedback/` paths.
`internal/claude` failed; one worker then reported the failure as "pre-existing on main" because
the breaking commit was already merged in the same session. Cost: three extra turns
(`f07f1c4` → `debc5ab`).

The validation itself works correctly; only its discoverability failed.

## Notes

- `CLAUDE.md`'s "Docs Layout" section is where an agent already looks before choosing a doc's
  home, and is the cheapest place to state the constraint in one line with a pointer to the
  contract.
- The rule could also be surfaced at the point of failure: the validation error names the
  offending reference but not the fix (summarize inline, or use a stable external link).
- Consider whether the copyable-doc contract belongs in a doc about docs rather than in
  `CLIDesign.md`; keep one normative home, not two.
