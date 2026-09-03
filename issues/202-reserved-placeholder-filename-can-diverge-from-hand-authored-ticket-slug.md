# 202 — Reserved placeholder filename can diverge from hand-authored ticket slug

**Status**: Closed — resolved in c4d6a67
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

## 4. Resolution

Implemented remediation option 1, the smaller of the two proposed fixes: `harnez find
issues next --reserve` now prints the exact reserved filename in its non-JSON output
(`cmd/harnez/find.go`, `runFindNext`), as `<NUMBER><TAB>issues/<reserved-filename>.md`,
instead of just the bare ticket number. The JSON form already carried `file`/`path`
fields (issue 194); only the plain-text form was missing this. Callers that write ticket
content directly to the printed path can no longer diverge from the slug Reserve()
actually picked, removing the divergence opportunity at the source rather than trying to
detect it after the fact. `docs/practices/IssueTracking.md` was updated to tell agents to
use the printed path rather than re-deriving a slug from the title by hand.

Added `TestRunFind_IssuesNextReservePrintsExactFilename` (`cmd/harnez/find_test.go`),
which reserves a placeholder for a title ("Add Doubled-Res. Sparklines!") whose
plausible hand-derived slug ("add-doubled-resolution-sparklines") differs from the slug
`Slugify` actually produces ("add-doubled-res-sparklines"), then asserts the path printed
by `--reserve` exists on disk while the hand-derived guess does not -- reproducing and
resolving the exact scenario from issue 202. Existing reserve tests
(`TestRunFind_IssuesNextReserve`) were updated for the new tab-separated output format.

`go test ./...` passes. `make install` run after the Go change.
