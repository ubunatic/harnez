# 916 — Study commits fail on studies index drift; offer a one-call fix

**Status**: Closed — harnez index --stage regenerates and stages only index files it rewrote (ac726732); studies drift lint names it. Developer make test-q1 green and scratch-repo check (exactly study + docs/README.md staged); host diff review and live no-op check. Known limit: a file rewritten by an earlier run and left unstaged is not staged.
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: 148 (harnez index), docs/studies/2026-10-25-session-retro-harnez-epics-910-911.md (proposal 4)

---

/goal Adding a study no longer costs a failed commit: either the commit lint's error names the exact fix command
or a harnez verb adds the study and the regenerated `docs/README.md` in one call, with a test; or stop and report
on a design choice (hook auto-staging vs. explicit verb). Circuit breaker: on a loop or no progress, clear the
goal and write a failure report here.

Seen several times in sessions 2026-10-23 to 2026-10-25: the first study commit fails on `docs/README.md` index
drift, the host runs `harnez index`, stages and recommits. Check first what the lint message says today.

## Decision (host, epic 939, 2026-10-25)

Lint already names `harnez index` (`internal/issues/maintenance/rebase.go:586`); the cost is the extra
stage-and-recommit round. Add an explicit `harnez index --stage` that regenerates and `git add`s only the index
files it rewrote, and make the lint message name that one command. No auto-staging pre-commit hook (hidden side
effects). Reversible.