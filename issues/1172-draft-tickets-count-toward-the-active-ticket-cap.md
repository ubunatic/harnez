# 1172 — Draft tickets count toward the active-ticket cap

**Status**: Closed — works as designed: spec/issues.yaml lists Draft with filters [draft, active], and warnActiveTicketCap in internal/issues/maintenance/rebase.go counts active := open + draft deliberately (843 M3 made Draft part of the active index); Jules audit PR github#149, reports/jules/1172.md
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: 843, 1140-1170; study docs/studies/2026-11-06-jules-backlog-cleanup.md

---

## Problem

After filing 31 `Draft` wrapper tickets (1140-1170) on 2026-11-06, every commit warned
`62 active tickets exceeds configured cap 30 (.harnez/repo.yaml)`, up from 31. Drafts are
placeholders, not started work, so counting them makes the cap warning noise during batch filing.
Not verified: whether counting Drafts is intended by 843.

## Want

- Decide whether `Draft` counts as active; if not, exclude it from the cap count, with a test.
