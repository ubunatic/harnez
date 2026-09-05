# 240 — Duplicate ticket numbers 179 and 180 in issues/README.md

**Status**: Open — harnez status catch during 2026-09-05 evergreen review
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Documentation
**Related**: `issues/179-harnez-rate-ok-heartbeat.md`,
`issues/179-instruct-agents-on-harnez-find-in-agents-md.md`,
`issues/180-release-webextension-manifest-and-json-version-sync.md`,
`issues/180-watch-and-review-compact-commands.md`, IssueTracking.md §4.1 lifecycle
invariant #4 ("No duplicate ticket numbers exist")

---

## 1. Problem & Motivation

`harnez status`'s tracker linter flags two duplicate ticket numbers, found during the
2026-09-05 `/evergreen` review:

- **179**: both `179-harnez-rate-ok-heartbeat.md` and
  `179-instruct-agents-on-harnez-find-in-agents-md.md` exist.
- **180**: both `180-release-webextension-manifest-and-json-version-sync.md` and
  `180-watch-and-review-compact-commands.md` exist.

All four tickets are already `Closed`, so this is a historical numbering collision (two
concurrent filing passes must have raced or a manual number was picked by hand) rather
than an active-work conflict. Low urgency, but it violates the tracker's own documented
invariant and will keep tripping `harnez status`'s linter on every run until fixed.

## 2. Technical Specification / Findings

Renumbering a closed ticket is mechanical but not zero-risk: any other ticket's
`**Related**:` line, commit message, or doc that references e.g. "issue 180" by number
alone (not by filename) could go stale after a rename. A grep across `issues/*.md` and
`docs/**/*.md` for bare references to `179`/`180` should be done before renaming, not
just a filename move.

## 3. Implementation & Verification Plan

Not yet planned. Likely shape: pick one ticket in each colliding pair to keep its number
and renumber the other to the next free number (`harnez find issues next`), updating its
filename, its own `# NNN — Title` header, and its `issues/README.md` row. Verify with
`harnez status` (duplicate diagnostic gone) and `harnez index --check` (clean).
