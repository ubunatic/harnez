# 471 — Root docs copies drift from copyable sources (Bash, Make, IssueTracking, Spec); reconcile with source-wins rule

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Docs / Tooling
**Related**: [353](353-tell-only-claude-never-propose-claude-md-changes-assume-other-agents.md), `AGENTS.md` ("Where Repo Rules Go, and Copyable-Doc Sources"), `harnez init`

---

## Problem

`harnez init` copies `docs/lang|practices|other/*.md` over the root `docs/*.md`. On 2026-09-21 a run
rewrote `docs/Bash.md` (~300 lines), `docs/IssueTracking.md` (~190), `docs/Make.md` (~120),
`docs/Spec.md` (5) and `docs/GoRelease.md` (~340), so the root copies differ substantially from
their sources. It is unknown which side is newer: root-only edits may exist that the sources lack,
or the sources may simply be ahead. AgenticLoop had root-only edits and was reconciled by a
three-way merge (`8f0e9a3`). Note `docs/lang/GoRelease.md` does not exist, so GoRelease's source
location needs finding first.

## /goal

For each drifted doc, decide per hunk whether the source or the root copy is right, port any
root-only content into the source (source wins going forward), then make `harnez init -d .` a
no-op on `docs/*.md`. Consider a guard (`harnez status` or a test) that reports root-only edits
before `init` overwrites them.

## Notes

- Use a three-way merge with the last commit where root and source agreed as the base.
- Never run `init` and commit blindly: check `git diff` for every rewritten doc.
- Re-verify against live state before starting.

- 2026-09-24 (519 sprint): `harnez init -d .` after `make install` also rewrote `docs/GoRelease.md`; the drift now covers Bash, GoRelease, IssueTracking, Make and Spec. Reverted each time with `git checkout --`.
