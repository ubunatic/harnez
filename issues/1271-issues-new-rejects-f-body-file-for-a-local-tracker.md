# 1271 — issues new rejects -F/--body-file for a local tracker

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: 1202

---

## 1. Problem & Motivation

Reported by the splitty host session (taia, 2026-11-23): in a repo with a local `issues/`
tracker, `harnez issues new "T" -F body.md` fails with
`-F/--body-file and --label apply only when tickets live on the forge`. An agent cannot supply
the ticket body at creation; it has to create the skeleton and then overwrite the file, a second
write that can drop the generated header if done carelessly.

## 2. Technical Specification / Findings

- The forge path already accepts `-F` (`runIssuesNewForge`); the local path refuses it.

## 3. Implementation & Verification Plan

- Local `issues new -F <file>`: keep the generated header (number, title, Status, Priority,
  Severity, Category, Related) and replace the template sections with the file's content.
- Decide whether `--label` maps to a local field or stays forge-only with a clearer error.
- Test: local `new -F` writes header plus body; an empty or whitespace-only file is refused, as
  on the forge path.
