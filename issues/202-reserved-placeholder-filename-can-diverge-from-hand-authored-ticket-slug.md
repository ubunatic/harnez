# 202 — Reserved placeholder filename can diverge from hand-authored ticket slug

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: harnez feedback entry `1607041e` (logged from `smarthome` project, 2026-09-03)

---

## 1. Problem & Motivation

`harnez find -d . issues next --reserve "<title>"` reserves a placeholder ticket at a
filename slug auto-derived from the title. If the agent later writes the real ticket
content to a filename it derives itself from the same title by hand, the two slugs can
differ. The reserved placeholder stub is then left behind, unfilled, at its own path.
`harnez index` subsequently treats the orphaned placeholder as a distinct ticket and
materializes a duplicate entry in `issues/README.md`.

## 2. Technical Specification / Findings

Reported via `harnez feedback issue` (severity: instruction) while filing tickets
015 and (nearly) 016–018 in the `smarthome` project session. Hit twice in one session,
suggesting the divergence is easy to trigger, not a one-off typo.

Workaround used in the field: after reserving, run `ls issues/ | grep "^0XX"` to find
the exact reserved filename and write ticket content directly to that path, rather
than re-deriving the filename from the title.

## 3. Implementation & Verification Plan

- Make `--reserve` print (or otherwise surface) the exact reserved filename so callers
  never need to re-derive or grep for it.
- Alternatively/additionally, have `harnez index` detect an orphaned reserved
  placeholder whose number has no matching filled ticket and warn instead of silently
  listing it as a separate open issue.
- Verify by reproducing the original divergence (reserve with one title, write to a
  differently-derived slug) and confirming index no longer double-lists it.
